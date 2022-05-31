package importer

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/antchfx/xpath"
	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/xlsxgen"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
)

// metaName defines the meta data of each worksheet.
const (
	metaName             = "TABLEAU"
	emptyMetaSheetRegexp = `^\s*<!--\s*@TABLEAU\s*-->\s*$` // e.g.: <!-- @TABLEAU -->
)

type Pass int

const (
	firstPass  Pass = 1
	secondPass Pass = 2
)

type Range struct {
	begin   int // index that Range begins at
	attrNum int // the number of attrs with the same prefix
	len     int // total number of columns with the same prefix, including attrs and children
}
type XMLImporter struct {
	filename   string
	sheetMap   map[string]*book.Sheet        // sheet name -> sheet
	metaMap    map[string]*xlsxgen.MetaSheet // sheet name -> meta
	sheetNames []string

	prefixMaps map[string](map[string]Range) // sheet -> { prefix -> [6, 9) }
}

type NoNeedParseError struct {
	err error
}

func (e NoNeedParseError) Error() string {
	return "`@TABLEAU` not found"
}

func (e NoNeedParseError) Unwrap() error {
	return e.err
}

var metaBeginRegexp *regexp.Regexp
var metaEndRegexp *regexp.Regexp
var attrValRegexp *regexp.Regexp

func init() {
	metaBeginRegexp = regexp.MustCompile(`^\s*<!--\s*@TABLEAU\s*$|` + emptyMetaSheetRegexp) // e.g.: <!--    @TABLEAU
	metaEndRegexp = regexp.MustCompile(`^\s*-->\s*$|` + emptyMetaSheetRegexp)               // e.g.:       -->
	attrValRegexp = regexp.MustCompile(`"` + types.PubTypeGroup + `"`)                                             // e.g.: "map<uint32, Type>"
}

func matchMetaBeginning(s string) []string {
	return metaBeginRegexp.FindStringSubmatch(s)
}

func isMetaBeginning(s string) bool {
	return matchMetaBeginning(s) != nil
}

func matchMetaEnding(s string) []string {
	return metaEndRegexp.FindStringSubmatch(s)
}

func isMetaEnding(s string) bool {
	return matchMetaEnding(s) != nil
}

// getMetaDoc get metaSheet document from `@TABLEAU` comments block. e.g.:
//
// <!-- @TABLEAU
// <ServerConf key="map<uint32,ServerConf> Open="bool">
// 	...
// </ServerConf>
// -->
//
// will be converted to
//
// <ServerConf key="map<uint32,ServerConf> Open="bool">
// 	...
// </ServerConf>
func getMetaDoc(doc string) (metaDoc string, err error) {
	metaBuf := bytes.NewBuffer(make([]byte, 0, len(doc)))
	scanner := bufio.NewScanner(strings.NewReader(doc))
	inMetaBlock := false
	foundMeta := false
	for scanner.Scan() {
		metaBeginning := isMetaBeginning(scanner.Text())
		metaEnding := isMetaEnding(scanner.Text()) && (inMetaBlock || metaBeginning)
		if metaBeginning {
			foundMeta = true
		}
		// close a meta block
		if metaEnding {
			break
		}
		if metaBeginning && !metaEnding {
			inMetaBlock = true
		} else if inMetaBlock {
			metaBuf.WriteString(scanner.Text() + "\n")
		}
	}
	// `@TABLEAU` must exist
	if !foundMeta {
		return metaBuf.String(), &NoNeedParseError{}
	}
	return metaBuf.String(), nil
}

// escapeMetaDoc escape characters for all attribute values in the document. e.g.:
//
// <ServerConf key="map<uint32,ServerConf> Open="bool">
// 	...
// </ServerConf>
//
// will be converted to
//
// <ServerConf key="map&lt;uint32,ServerConf&gt; Open="bool">
// 	...
// </ServerConf>
func escapeAttrs(doc string) string {
	escapedDoc := attrValRegexp.ReplaceAllStringFunc(doc, func(s string) string {
		var buf bytes.Buffer
		xml.EscapeText(&buf, []byte(s[1:len(s)-1]))
		return fmt.Sprintf("\"%s\"", buf.String())
	})
	return escapedDoc
}

// contains check if sheets contains a specific sheet
func contains(sheets []string, sheet string) bool {
	for _, s := range sheets {
		if sheet == s {
			return true
		}
	}
	return false
}

// TODO: options
func NewXMLImporter(filename string, sheets []string) (*XMLImporter, error) {
	return &XMLImporter{
		filename:   filename,
		sheetNames: sheets,
		prefixMaps: make(map[string](map[string]Range)),
	}, nil
}

func (x *XMLImporter) BookName() string {
	return strings.TrimSuffix(filepath.Base(x.filename), filepath.Ext(x.filename))
}

func (x *XMLImporter) Filename() string {
	return x.filename
}

func (x *XMLImporter) GetSheets() []*book.Sheet {
	if x.sheetNames == nil {
		if err := x.parse(); err != nil {
			atom.Log.Panicf("failed to parse: %s, %+v", x.filename, err)
		}
	}

	sheets := []*book.Sheet{}
	for _, name := range x.sheetNames {
		sheet := x.GetSheet(name)
		if sheet == nil {
			atom.Log.Panicf("get sheet failed: %s", name)
		}
		sheets = append(sheets, sheet)
	}
	return sheets
}

// GetSheet returns a Sheet of the specified sheet name.
func (x *XMLImporter) GetSheet(name string) *book.Sheet {
	if x.sheetMap == nil {
		if err := x.parse(); err != nil {
			atom.Log.Panicf("failed to parse: %s, %+v", x.filename, err)
		}
	}

	if sheet, ok := x.sheetMap[name]; !ok {
		atom.Log.Panicf("get sheet failed: %s", name)
	} else {
		return sheet
	}
	return nil
}

func (x *XMLImporter) parse() error {
	x.sheetMap = make(map[string]*book.Sheet)
	x.metaMap = make(map[string]*xlsxgen.MetaSheet)
	x.sheetNames = nil

	// open xml file and parse the document
	xmlPath := x.filename
	atom.Log.Debugf("xml: %s", xmlPath)
	buf, err := os.ReadFile(xmlPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", xmlPath)
	}

	// get metaSheet document
	metaDoc, err := getMetaDoc(string(buf))
	if err != nil {
		var noNeedParse *NoNeedParseError
		if errors.As(err, &noNeedParse) {
			atom.Log.Infof("%s no need parse: %s", xmlPath, noNeedParse)
			return nil
		} else {
			return errors.Wrapf(err, "failed to getMetaDoc from xml content:\n%s", string(buf))
		}
	}

	// escape characters for attribute
	metaDoc = escapeAttrs(metaDoc)
	atom.Log.Debug(metaDoc)

	//------------------------------ The first pass ------------------------------//	
	// parse the metaSheet
	// Note that one xml file only has one root
	// So in order to have multiple roots, we need to use a stream parser
	hasUserSheets := x.sheetNames != nil
	p, err := xmlquery.CreateStreamParser(strings.NewReader(metaDoc), "/")
	if err != nil {
		return errors.Wrapf(err, "failed to create stream parser from string %s", metaDoc)
	}
	for {
		n, err := p.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrapf(err, "failed to read from stream parser")
		}
		nav := xmlquery.CreateXPathNavigator(n)
		if !hasUserSheets {
			x.sheetNames = append(x.sheetNames, nav.LocalName())
		}
		if err := x.parseSheet(nav.Current(), nav.LocalName(), firstPass, true); err != nil {
			return errors.WithMessagef(err, "failed to parse `@%s` sheet: %s#%s", metaName, x.filename, nav.LocalName())
		}
	}
	atom.Log.Debug(x.sheetNames)
	atom.Log.Debug(string(buf))

	// parse data sheets
	p, err = xmlquery.CreateStreamParser(strings.NewReader(string(buf)), "/")
	if err != nil {
		return errors.Wrapf(err, "failed to create stream parser from string %s", metaDoc)
	}
	for {
		n, err := p.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrapf(err, "failed to read from stream parser")
		}
		// parse sheets
		if contained := contains(x.sheetNames, n.Data); !hasUserSheets || contained {
			if err := x.parseSheet(n, n.Data, firstPass, false); err != nil {
				return errors.WithMessagef(err, "failed to parse sheet: %s#%s", x.filename, n.Data)
			}
			if !hasUserSheets && !contained {
				x.sheetNames = append(x.sheetNames, n.Data)
			}
		}
	}
	atom.Log.Debug(x.sheetNames)

	//------------------------------ The second pass ------------------------------//
	// only parse data sheets
	p, _ = xmlquery.CreateStreamParser(strings.NewReader(string(buf)), "/")
	for {
		n, err := p.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrapf(err, "failed to read from stream parser")
		}
		// parse sheets
		if contains(x.sheetNames, n.Data) {
			if err := x.parseSheet(n, n.Data, secondPass, false); err != nil {
				return errors.WithMessagef(err, "failed to parse sheet: %s#%s", x.filename, n.Data)
			}
		}
	}

	return nil
}

func (x *XMLImporter) parseSheet(doc *xmlquery.Node, sheetName string, pass Pass, isMeta bool) error {
	// In order to combine column headers (the result of 1 pass) and data (the result of 2 pass),
	// we need to cache the MetaSheet struct in `x`
	metaSheet, ok := x.metaMap[sheetName]
	header := options.NewDefault().Input.Proto.Header
	if !ok {
		metaSheet = xlsxgen.NewMetaSheet(sheetName, header, false)
		x.metaMap[sheetName] = metaSheet
		x.prefixMaps[sheetName] = make(map[string]Range)
	}
	root := xmlquery.CreateXPathNavigator(doc)
	switch pass {
	case firstPass:
		// 1 pass: scan all columns and their types
		if err := x.parseNodeType(root, metaSheet, isMeta, true); err != nil {
			return errors.Wrapf(err, "failed to parseNodeType for root node %s", sheetName)
		}
	case secondPass:
		// 2 pass: fill data to the corresponding columns
		if err := x.parseNodeData(root, metaSheet, int(metaSheet.Datarow)-1); err != nil {
			return errors.Wrapf(err, "failed to parseNodeData for root node %s", sheetName)
		}
	}
	// atom.Log.Info(metaSheet)
	if pass == secondPass {
		// unpack rows from the MetaSheet struct
		var rows [][]string
		for i := 0; i < len(metaSheet.Rows); i++ {
			var row []string
			for _, cell := range metaSheet.Rows[i].Cells {
				row = append(row, cell.Data)
			}
			rows = append(rows, row)
		}
		// insert sheets into map for importer
		sheet := book.NewSheet(sheetName, rows)
		sheet.Meta = &tableaupb.SheetMeta{
			Sheet:    sheetName,
			Alias:    sheetName,
			Namerow:  header.Namerow,
			Typerow:  header.Typerow,
			Noterow:  header.Noterow,
			Datarow:  header.Datarow,
			Nameline: 1,
			Typeline: 1,
			Nested:   true,
		}
		x.sheetMap[sheetName] = sheet
	}
	return nil
}

// parseNodeType parse and convert an xml file to sheet format
func (x *XMLImporter) parseNodeType(nav *xmlquery.NodeNavigator, metaSheet *xlsxgen.MetaSheet, isMeta, isFirstChild bool) error {
	// preprocess
	prefix := ""
	continueFindNude := true
	var parentList []string	
	var nudeParentTypeList []string
	// construct prefix
	for flag, navCopy := true, *nav; flag && navCopy.LocalName() != metaSheet.Worksheet; flag = navCopy.MoveToParent() {
		if prefix != "" && continueFindNude {
			if len(navCopy.Current().Attr) > 0 {
				continueFindNude = false
			} else {
				t := fmt.Sprintf("{%s}", navCopy.LocalName())
				if navCopy.Current().Parent != nil && len(xmlquery.Find(navCopy.Current().Parent, navCopy.LocalName())) > 1 {
					t = fmt.Sprintf("[%s]", navCopy.LocalName())
				}
				nudeParentTypeList = append(nudeParentTypeList, t)				
			}
		}
		prefix = strcase.ToCamel(navCopy.LocalName()) + prefix
		parentList = append(parentList, navCopy.LocalName())
	}
	repeated := len(xmlquery.Find(nav.Current().Parent, nav.LocalName())) > 1

	// iterate over attributes
	for i, attr := range nav.Current().Attr {
		attrName := attr.Name.Local
		attrValue := attr.Value
		_, prefixExist := x.prefixMaps[metaSheet.Worksheet][prefix]
		x.tryAddCol(metaSheet, parentList, strcase.ToCamel(attrName))

		t, d := inferType(attrValue)
		colName := prefix + strcase.ToCamel(attrName)
		metaSheet.SetDefaultValue(colName, d)
		if isMeta {
			if index := strings.Index(attrValue, "|"); index > 0 {
				t = attrValue[:index]
				metaSheet.SetDefaultValue(colName, attrValue[index+1:])
			} else {
				t = attrValue
			}
		}

		// atom.Log.Debug(t)		
		curType := metaSheet.GetColType(colName)
		matches := types.MatchStruct(curType)
		// 1. <TABLEAU>
		// 2. type not set
		// 3. {Type}int32 -> [Type]int32 (when mistaken it as a struct at first but discover multiple elements later)
		needChangeType := isMeta || curType == "" || (len(matches) > 0 && repeated)
		// 1. new struct(list), not subsequent
		// 2. {Type}int32 -> [Type]int32 (when mistaken it as a struct at first but discover multiple elements later)
		setKeyedType := (!prefixExist || (len(matches) > 0 && repeated)) && nav.LocalName() != metaSheet.Worksheet
		if needChangeType {
			oldType := t
			typePrefix :=  ""
			for _, parentType := range nudeParentTypeList {
				typePrefix = parentType + typePrefix
			}
			t = typePrefix + t
			// atom.Log.Debug(typePrefix)
			if matches := types.MatchMap(t); len(matches) >= 3 {
				// case 1: map<uint32,Type>
				if !types.IsScalarType(matches[1]) && len(types.MatchEnum(matches[1])) == 0 {
					return errors.Errorf("%s is not scalar type in node %s attr %s type %s", matches[1], nav.LocalName(), attrName, t)
				}
				if strings.TrimSpace(matches[2]) != nav.LocalName() {
					return errors.Errorf("%s in attr %s type %s must be the same as node name %s", matches[2], attrName, t, nav.LocalName())
				}
				metaSheet.SetColType(colName, t)
			} else if matches := types.MatchKeyedList(t); len(matches) >= 3 {
				// case 2: [Type]<uint32>
				if i != 0 {
					return errors.Errorf("KeyedList attr %s in node %s must be the first attr", attrName, nav.LocalName())
				}
				if !types.IsScalarType(matches[2]) && len(types.MatchEnum(matches[2])) == 0 {
					return errors.Errorf("%s is not scalar type in node %s attr %s type %s", matches[2], nav.LocalName(), attrName, t)
				}
				if strings.TrimSpace(matches[1]) != nav.LocalName() {
					return errors.Errorf("%s in attr %s type %s must be the same as node name %s", matches[1], attrName, t, nav.LocalName())
				}
				metaSheet.SetColType(colName, t)
			} else if matches := types.MatchList(t); len(matches) >= 3 {
				// case 3: [Type]uint32
				if i != 0 {
					return errors.Errorf("KeyedList attr %s in node %s must be the first attr", attrName, nav.LocalName())
				}
				if !types.IsScalarType(matches[2]) && len(types.MatchEnum(matches[2])) == 0 {
					return errors.Errorf("%s is not scalar type in node %s attr %s type %s", matches[2], nav.LocalName(), attrName, t)
				}
				if strings.TrimSpace(matches[1]) != nav.LocalName() {
					return errors.Errorf("%s in attr %s type %s must be the same as node name %s", matches[1], attrName, t, nav.LocalName())
				}
				metaSheet.SetColType(colName, t)
			} else if i == 0 && setKeyedType {
				t = oldType
				// case 4: {Type}uint32
				if repeated {
					metaSheet.SetColType(colName, fmt.Sprintf("%s[%s]<%s>", typePrefix, strcase.ToCamel(nav.LocalName()), t))
				} else {
					metaSheet.SetColType(colName, fmt.Sprintf("%s{%s}%s", typePrefix, strcase.ToCamel(nav.LocalName()), t))
				}
			} else {
				t = oldType
				// default: built-in type
				metaSheet.SetColType(colName, t)
			}
		}
	}

	// iterate over child nodes
	navCopy := *nav
	for flag, i := navCopy.MoveToChild(), 0; flag; flag = navCopy.MoveToNext() {
		// commentNode, documentNode and other meaningless nodes should be filtered
		if navCopy.NodeType() != xpath.ElementNode {
			continue
		}
		tagName := navCopy.LocalName()
		if err := x.parseNodeType(&navCopy, metaSheet, isMeta, i == 0); err != nil {
			return errors.Wrapf(err, "failed to parseNodeType for the node %s", tagName)
		}
		i++
	}

	return nil
}

// parseNodeData parse and convert an xml file to sheet format
func (x *XMLImporter) parseNodeData(nav *xmlquery.NodeNavigator, metaSheet *xlsxgen.MetaSheet, cursor int) error {
	// preprocess
	prefix := ""
	// construct prefix
	for flag, navCopy := true, *nav; flag && navCopy.LocalName() != metaSheet.Worksheet; flag = navCopy.MoveToParent() {
		prefix = strcase.ToCamel(navCopy.LocalName()) + prefix
	}

	// clear to the bottom
	if navCopy := *nav; !navCopy.MoveToChild() {
		for tmpCusor := cursor; tmpCusor < len(metaSheet.Rows); tmpCusor++ {
			metaSheet.ForEachCol(tmpCusor, func(name string, cell *xlsxgen.Cell) error {
				if strings.HasPrefix(name, prefix) {
					cell.Data = metaSheet.GetDefaultValue(name)
				}
				return nil
			})
		}
	}

	// iterate over attributes
	for _, attr := range nav.Current().Attr {
		attrName := attr.Name.Local
		attrValue := attr.Value
		colName := prefix + strcase.ToCamel(attrName)
		// fill values to the bottom when backtrace to top line
		for tmpCusor := cursor; tmpCusor < len(metaSheet.Rows); tmpCusor++ {
			metaSheet.Cell(tmpCusor, len(metaSheet.Rows[metaSheet.Namerow-1].Cells), colName).Data = attrValue
		}
	}

	// iterate over child nodes
	nodeMap := make(map[string]int)
	navCopy := *nav
	for flag := navCopy.MoveToChild(); flag; flag = navCopy.MoveToNext() {
		// commentNode, documentNode and other meaningless nodes should be filtered
		if navCopy.NodeType() != xpath.ElementNode {
			continue
		}
		tagName := navCopy.LocalName()
		if count, existed := nodeMap[tagName]; existed {
			// duplicate means a list, should expand vertically
			row := metaSheet.NewRow()
			if err := x.parseNodeData(&navCopy, metaSheet, row.Index); err != nil {
				return errors.Wrapf(err, "parseNodeData for node %s (index:%d) failed", tagName, count+1)
			}
			nodeMap[tagName]++
		} else {
			if err := x.parseNodeData(&navCopy, metaSheet, cursor); err != nil {
				return errors.Wrapf(err, "parseNodeData for the first node %s failed", tagName)
			}
			nodeMap[tagName] = 1
		}
	}

	return nil
}

// tryAddCol add a new column named `name` to an appropriate location in metaSheet if not exists or do nothing otherwise
func (x *XMLImporter) tryAddCol(metaSheet *xlsxgen.MetaSheet, parentList []string, name string) {
	prefix := ""
	var reversedList []string
	parentMap := make(map[string]bool)
	prefixMap := x.prefixMaps[metaSheet.Worksheet]
	for i := len(parentList) - 1; i >= 0; i-- {
		prefix += parentList[i]
		parentMap[prefix] = true
		if i > 0 {
			reversedList = append(reversedList, prefix)
		}
	}
	colName := prefix + name
	if metaSheet.HasCol(colName) {
		return
	}
	shift := func(r Range) {
		for i := 0; i < len(reversedList); i++ {
			if r, ok := prefixMap[reversedList[i]]; ok {
				prefixMap[reversedList[i]] = Range{r.begin, r.attrNum, r.len + 1}
			}
		}
		for k, v := range prefixMap {
			if _, ok := parentMap[k]; !ok && v.begin > r.begin {
				prefixMap[k] = Range{v.begin + 1, v.attrNum, v.len}
			}
		}
	}
	// insert prefixMap
	if r, ok := prefixMap[prefix]; !ok {
		index := len(metaSheet.Rows[metaSheet.Namerow-1].Cells)
		if len(reversedList) > 0 {
			parentPrefix := reversedList[len(reversedList)-1]
			if r2, ok := prefixMap[parentPrefix]; ok {
				index = r2.begin + r2.len
			}
		}
		prefixMap[prefix] = Range{index, 1, 1}
		shift(prefixMap[prefix])
		metaSheet.Cell(int(metaSheet.Namerow)-1, prefixMap[prefix].begin, colName).Data = colName
	} else {
		prefixMap[prefix] = Range{r.begin, r.attrNum + 1, r.len + 1}
		shift(prefixMap[prefix])
		metaSheet.Cell(int(metaSheet.Namerow)-1, r.begin+r.attrNum, colName).Data = colName
	}
}

func inferType(value string) (string, string) {
	var t, d string
	if _, err := strconv.Atoi(value); err == nil {
		t, d = "int32", "0"
	} else if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		t, d = "int64", "0"
	} else {
		t, d = "string", ""
	}
	return t, d
}
