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

type Range struct {
	begin   int // index that Range begins at
	attrNum int // the number of attrs with the same prefix
	len     int // total number of columns with the same prefix, including attrs and children
}
type XMLImporter struct {
	*book.Book
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

// TODO: options
func NewXMLImporter(filename string, sheets []string) (*XMLImporter, error) {
	newBook, err := parseXML(filename, sheets)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse xml:%s", filename)
	}
	if newBook == nil {
		atom.Log.Debugf("xml:%s parsed to an empty book", filename)
		bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
		return &XMLImporter{
			Book: book.NewBook(bookName, filename, nil),
		}, nil
	}
	// newBook.ExportCSV()
	
	return &XMLImporter{
		Book: newBook,
	}, nil
}

// parseXML parse sheets in the XML file named `filename` and return a book with multiple sheets
// in TABLEAU grammar which can be exported to protobuf by excel parser.
func parseXML(filename string, sheetNames []string) (*book.Book, error) {
	// open xml file and parse the document
	xmlPath := filename
	atom.Log.Debugf("xml: %s", xmlPath)
	buf, err := os.ReadFile(xmlPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %s", xmlPath)
	}

	// get metaSheet document
	metaDoc, err := getMetaDoc(string(buf))
	if err != nil {
		var noNeedParse *NoNeedParseError
		if errors.As(err, &noNeedParse) {
			atom.Log.Debugf("%s no need parse: %s", xmlPath, noNeedParse)
			return nil, nil
		} else {
			return nil, errors.Wrapf(err, "failed to getMetaDoc from xml content:\n%s", string(buf))
		}
	}

	// escape characters for attribute
	metaDoc = escapeAttrs(metaDoc)
	atom.Log.Debug(metaDoc)

	//------------------------------ The first pass ------------------------------//	
	// parse the metaSheet
	// Note that one xml file only has one root
	// So in order to have multiple roots, we need to use a stream parser
	noSheetByUser := len(sheetNames) == 0
	header := options.NewDefault().Input.Proto.Header
	type SheetCache struct{
		metaSheet *xlsxgen.MetaSheet // metaSheet begins with `<-- @TABLEAU`
		prefixMap map[string]Range // prefix -> [6, 9)
	}
	sheetMap := make(map[string]*SheetCache)
	p, err := xmlquery.CreateStreamParser(strings.NewReader(metaDoc), "/")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create stream parser from string %s", metaDoc)
	}
	for {
		n, err := p.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read from stream parser")
		}		
		nav := xmlquery.CreateXPathNavigator(n)
		sheetName := nav.LocalName()
		// add new sheet to cache
		sheetMap[sheetName] = &SheetCache{xlsxgen.NewMetaSheet(sheetName, header, false), make(map[string]Range)}
		if err := firstParseSheet(nav, sheetMap[sheetName].metaSheet, sheetMap[sheetName].prefixMap, true); err != nil {
			return nil, errors.Wrapf(err, "failed to parse `@%s` sheet: %s#%s", metaName, filename, sheetName)
		}
		// append if user not specified
		if noSheetByUser {
			sheetNames = append(sheetNames, sheetName)
		}
	}
	atom.Log.Debug(sheetNames)

	// parse data sheets
	noSheetInMeta := len(sheetNames) == 0
	p, err = xmlquery.CreateStreamParser(strings.NewReader(string(buf)), "/")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create stream parser from string %s", metaDoc)
	}
	for {
		n, err := p.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read from stream parser")
		}
		nav := xmlquery.CreateXPathNavigator(n)
		sheetName := nav.LocalName()
		// add new sheet if not exist
		if _, ok := sheetMap[sheetName]; !ok {
			sheetMap[sheetName] = &SheetCache{xlsxgen.NewMetaSheet(sheetName, header, false), make(map[string]Range)}
		}
		if err := firstParseSheet(nav, sheetMap[sheetName].metaSheet, sheetMap[sheetName].prefixMap, false); err != nil {
			return nil, errors.Wrapf(err, "failed to parse sheet: %s#%s", filename, sheetName)
		}
		// generate all if metaSheet not specified
		if noSheetInMeta {
			sheetNames = append(sheetNames, sheetName)
		}
	}
	atom.Log.Debug(sheetNames)

	//------------------------------ The second pass ------------------------------//
	// only parse data sheets
	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(bookName, filename, nil)
	p, _ = xmlquery.CreateStreamParser(strings.NewReader(string(buf)), "/")
	for {
		n, err := p.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read from stream parser")
		}
		nav := xmlquery.CreateXPathNavigator(n)
		sheet, err := secondParseSheet(nav, sheetMap[nav.LocalName()].metaSheet)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse sheet: %s#%s", filename, nav.LocalName())
		}
		newBook.AddSheet(sheet)
	}
	newBook.Squeeze(sheetNames)

	return newBook, nil
}

// firstParseSheet do the jobs of first pass of the XML parser. To be more specified, recursively explore the document rooted by `root`
// and fill the header (first 2 rows) of `metaSheet`, which can fully describe the structure of document.
func firstParseSheet(root *xmlquery.NodeNavigator, metaSheet *xlsxgen.MetaSheet, prefixMap map[string]Range, isMeta bool) error {
	if err := parseNodeType(root, metaSheet, prefixMap, isMeta, true); err != nil {
		return errors.Wrapf(err, "failed to parseNodeType for root node %s", metaSheet.Worksheet)
	}
	atom.Log.Debug(metaSheet)
	return nil
}

// secondParseSheet proceed filling the data rows (begins from 5th row) of `metaSheet` on the basis of `firstParseSheet` and
// return a 2-dimensional book in TABLEAU grammar, which can fully describe both the structure and data of the document.
func secondParseSheet(root *xmlquery.NodeNavigator, metaSheet *xlsxgen.MetaSheet) (sheet *book.Sheet, err error) {
	if err := parseNodeData(root, metaSheet, int(metaSheet.Datarow)-1); err != nil {
		return nil, errors.Wrapf(err, "failed to parseNodeData for root node %s", metaSheet.Worksheet)
	}
	atom.Log.Debug(metaSheet)
	
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
	header := options.NewDefault().Input.Proto.Header
	sheet = book.NewSheet(metaSheet.Worksheet, rows)
	sheet.Meta = &tableaupb.SheetMeta{
		Sheet:    metaSheet.Worksheet,
		Alias:    metaSheet.Worksheet,
		Namerow:  header.Namerow,
		Typerow:  header.Typerow,
		Noterow:  header.Noterow,
		Datarow:  header.Datarow,
		Nameline: 1,
		Typeline: 1,
		Nested:   true,
	}
	return sheet, nil
}

// parseNodeType parse and convert an xml file to sheet format
func parseNodeType(nav *xmlquery.NodeNavigator, metaSheet *xlsxgen.MetaSheet, prefixMap map[string]Range, isMeta, isFirstChild bool) error {
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
		_, prefixExist := prefixMap[prefix]
		tryAddCol(metaSheet, parentList, prefixMap, strcase.ToCamel(attrName))

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

		atom.Log.Debug(t)		
		curType := metaSheet.GetColType(colName)
		matches := types.MatchStruct(curType)
		// 1. <TABLEAU>
		// 2. type not set
		// 3. {Type}int32 -> [Type]int32 (when mistaken it as a struct at first but discover multiple elements later)
		// NOTE: Map in struct not supported temporarily.
		needChangeType := isMeta || curType == "" || (len(matches) > 0 && repeated)
		// 1. new struct(list), not subsequent
		// 2. {Type}int32 -> [Type]int32 (when mistaken it as a struct at first but discover multiple elements later)
		setKeyedType := (!prefixExist || (len(matches) > 0 && repeated)) && nav.LocalName() != metaSheet.Worksheet
		if needChangeType {
			typePrefix :=  ""
			for _, parentType := range nudeParentTypeList {
				typePrefix = parentType + typePrefix
			}
			atom.Log.Debug(typePrefix)
			if matches := types.MatchMap(t); len(matches) >= 3 {
				t = typePrefix + t
				// case 1: map<uint32,Type>
				if !types.IsScalarType(matches[1]) && len(types.MatchEnum(matches[1])) == 0 {
					return errors.Errorf("%s is not scalar type in node %s attr %s type %s", matches[1], nav.LocalName(), attrName, t)
				}
				if strings.TrimSpace(matches[2]) != nav.LocalName() {
					return errors.Errorf("%s in attr %s type %s must be the same as node name %s", matches[2], attrName, t, nav.LocalName())
				}
				metaSheet.SetColType(colName, t)
			} else if matches := types.MatchKeyedList(t); len(matches) >= 3 {
				t = typePrefix + t
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
				t = typePrefix + t
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
				// case 4: {Type}uint32
				if repeated {
					metaSheet.SetColType(colName, fmt.Sprintf("%s[%s]<%s>", typePrefix, strcase.ToCamel(nav.LocalName()), t))
				} else {
					metaSheet.SetColType(colName, fmt.Sprintf("%s{%s}%s", typePrefix, strcase.ToCamel(nav.LocalName()), t))
				}
			} else {				
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
		if err := parseNodeType(&navCopy, metaSheet, prefixMap, isMeta, i == 0); err != nil {
			return errors.Wrapf(err, "failed to parseNodeType for the node %s", tagName)
		}
		i++
	}

	return nil
}

// parseNodeData parse and convert an xml file to sheet format
func parseNodeData(nav *xmlquery.NodeNavigator, metaSheet *xlsxgen.MetaSheet, cursor int) error {
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
			if err := parseNodeData(&navCopy, metaSheet, row.Index); err != nil {
				return errors.Wrapf(err, "parseNodeData for node %s (index:%d) failed", tagName, count+1)
			}
			nodeMap[tagName]++
		} else {
			if err := parseNodeData(&navCopy, metaSheet, cursor); err != nil {
				return errors.Wrapf(err, "parseNodeData for the first node %s failed", tagName)
			}
			nodeMap[tagName] = 1
		}
	}

	return nil
}

// tryAddCol add a new column named `name` to an appropriate location in metaSheet if not exists or do nothing otherwise
func tryAddCol(metaSheet *xlsxgen.MetaSheet, parentList []string, prefixMap map[string]Range, name string) {
	prefix := ""
	var reversedList []string
	parentMap := make(map[string]bool)
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
