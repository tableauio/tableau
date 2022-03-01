package importer

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/antchfx/xpath"
	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/xlsxgen"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
)

// metaName defines the meta data of each worksheet.
const (
	metaName         = "TABLEAU"
	placeholderName  = "_placeholder"
	placeholderType  = "bool"
	placeholderValue = "false"
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
	sheetMap   map[string]*Sheet             // sheet name -> sheet
	metaMap    map[string]*xlsxgen.MetaSheet // sheet name -> meta
	sheetNames []string
	header     *options.HeaderOption // header settings.

	prefixMaps map[string](map[string]Range) // sheet -> { prefix -> [6, 9) }
}

// TODO: options
func NewXMLImporter(filename string, sheets []string, header *options.HeaderOption) *XMLImporter {
	return &XMLImporter{
		filename:   filename,
		sheetNames: sheets,
		header:     header,
		prefixMaps: make(map[string](map[string]Range)),
	}
}

func (x *XMLImporter) GetSheets() ([]*Sheet, error) {
	if x.sheetNames == nil {
		if err := x.parse(); err != nil {
			return nil, errors.WithMessagef(err, "failed to parse %s", x.filename)
		}
	}

	sheets := []*Sheet{}
	for _, name := range x.sheetNames {
		sheet, err := x.GetSheet(name)
		if err != nil {
			return nil, errors.WithMessagef(err, "get sheet failed: %s", name)
		}
		sheets = append(sheets, sheet)
	}
	return sheets, nil
}

// GetSheet returns a Sheet of the specified sheet name.
func (x *XMLImporter) GetSheet(name string) (*Sheet, error) {
	if x.sheetMap == nil {
		if err := x.parse(); err != nil {
			return nil, errors.WithMessagef(err, "failed to parse %s", x.filename)
		}
	}

	sheet, ok := x.sheetMap[name]
	if !ok {
		return nil, errors.Errorf("sheet %s not found", name)
	}
	return sheet, nil
}

func (x *XMLImporter) parse() error {
	x.sheetMap = make(map[string]*Sheet)
	x.metaMap = make(map[string]*xlsxgen.MetaSheet)
	// open xml file and parse the document
	xmlPath := x.filename
	atom.Log.Debugf("xml: %s", xmlPath)
	buf, err := os.ReadFile(xmlPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", xmlPath)
	}
	// replacement for `<` and `>` not allowed in attribute values
	// e.g. `<ServerConf key="map<uint32,ServerConf> Open="bool">...</ServerConf>"`
	attrValRegexp := regexp.MustCompile(`"\S+"`)
	replacedStr := attrValRegexp.ReplaceAllStringFunc(string(buf), func(s string) string {
		var buf bytes.Buffer
		xml.EscapeText(&buf, []byte(s[1:len(s)-1]))
		return fmt.Sprintf("\"%s\"", buf.String())
	})
	// replacement for tableau keyword, which begins by `@`
	// e.g. `<@TABLEAU>``
	keywordRegexp := regexp.MustCompile(`([</]+)@([A-Z]+)`)
	replacedStr = keywordRegexp.ReplaceAllString(replacedStr, `$1$2`)
	// Note that one xml file only has one root
	// So in order to have multiple roots, we need to use a stream parser
	// The first pass
	hasMetaTag := false
	hasUserSheets := x.sheetNames != nil
	hasTableauSheets := false
	contains := func(sheets []string, sheet string) bool {
		for _, s := range sheets {
			if sheet == s {
				return true
			}
		}
		return false
	}
	p, err := xmlquery.CreateStreamParser(strings.NewReader(replacedStr), "/")
	if err != nil {
		return errors.Wrapf(err, "failed to create stream parser from string %s", replacedStr)
	}
	for {
		n, err := p.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrapf(err, "failed to read from stream parser")
		}
		// parse `<@TABLEAU>...</@TABLEAU>`
		if n.Data == metaName {
			hasMetaTag = true
			nav := xmlquery.CreateXPathNavigator(n)
			for flag := nav.MoveToChild(); flag; flag = nav.MoveToNext() {
				// commentNode, documentNode and other meaningless nodes should be filtered
				if nav.NodeType() != xpath.ElementNode {
					continue
				}
				if !hasUserSheets {
					x.sheetNames = append(x.sheetNames, nav.LocalName())
				}
				hasTableauSheets = true
				if err := x.parseSheet(nav.Current(), nav.LocalName(), firstPass); err != nil {
					return errors.WithMessagef(err, "failed to parse `@%s` sheet: %s#%s", metaName, x.filename, nav.LocalName())
				}
			}
		} else {
			// parse sheets
			if (!hasUserSheets && !hasTableauSheets) || contains(x.sheetNames, n.Data) {
				if err := x.parseSheet(n, n.Data, firstPass); err != nil {
					return errors.WithMessagef(err, "failed to parse sheet: %s#%s", x.filename, n.Data)
				}
				if !hasUserSheets && !hasTableauSheets {
					x.sheetNames = append(x.sheetNames, n.Data)
				}
			}
		}
		// `<@TABLEAU>...</@TABLEAU>` must be the first sheet
		if !hasMetaTag {
			atom.Log.Debugf("`<@TABLEAU>...</@TABLEAU>` not exists or not the first sheet")
			x.sheetNames = nil
			return nil
		}
	}

	// The second pass
	p, err = xmlquery.CreateStreamParser(strings.NewReader(replacedStr), "/")
	if err != nil {
		return errors.Wrapf(err, "failed to create stream parser from string %s", replacedStr)
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
		if contains(x.sheetNames, n.Data) {
			if err := x.parseSheet(n, n.Data, secondPass); err != nil {
				return errors.WithMessagef(err, "failed to parse sheet: %s#%s", x.filename, n.Data)
			}
		}
	}

	return nil
}

func (x *XMLImporter) parseSheet(doc *xmlquery.Node, sheetName string, pass Pass) error {
	// In order to combine column headers (the result of 1 pass) and data (the result of 2 pass),
	// we need to cache the MetaSheet struct in `x`
	metaSheet, exist := x.metaMap[sheetName]
	if !exist {
		metaSheet = xlsxgen.NewMetaSheet(sheetName, x.header, false)
		x.metaMap[sheetName] = metaSheet
		x.prefixMaps[sheetName] = make(map[string]Range)
	}
	root := xmlquery.CreateXPathNavigator(doc)
	isMeta := doc.Parent != nil && doc.Parent.Data == metaName
	switch pass {
	case firstPass:
		// 1 pass: scan all columns and their types
		if err := x.parseNodeType(root, metaSheet, isMeta); err != nil {
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
		sheet := NewSheet(sheetName, rows)
		sheet.Meta = &tableaupb.SheetMeta{
			Sheet:    sheetName,
			Alias:    sheetName,
			Nameline: 1,
			Typeline: 1,
			Nested:   true,
		}
		x.sheetMap[sheetName] = sheet
	}
	return nil
}

// parseNodeType parse and convert an xml file to sheet format
func (x *XMLImporter) parseNodeType(nav *xmlquery.NodeNavigator, metaSheet *xlsxgen.MetaSheet, isMeta bool) error {
	// preprocess
	prefix := ""
	var parentList []string
	// construct prefix
	for flag, navCopy := true, *nav; flag && navCopy.LocalName() != metaSheet.Worksheet; flag = navCopy.MoveToParent() {
		prefix = strcase.ToCamel(navCopy.LocalName()) + prefix
		parentList = append(parentList, navCopy.LocalName())
	}
	repeated := len(xmlquery.Find(nav.Current().Parent, nav.LocalName())) > 1

	// add placeholder to nude node
	if !isMeta && len(nav.Current().Attr) == 0 {
		colName := prefix + placeholderName
		x.tryAddCol(metaSheet, parentList, placeholderName)
		if nav.LocalName() != metaSheet.Worksheet {
			metaSheet.SetColType(colName, fmt.Sprintf("{%s}%s", strcase.ToCamel(nav.LocalName()), placeholderType))
		} else {
			metaSheet.SetColType(colName, placeholderType)
		}
	}

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
			if matches := types.MatchMap(t); len(matches) >= 3 {
				// case 1: map<uint32,Type>
				if !types.IsScalarType(matches[1]) && len(types.MatchEnum(matches[1])) == 0 {
					return errors.Errorf("%s is not scalar type in node %s attr %s type %s", matches[1], nav.LocalName(), attrName, t)
				}
				if matches[2] != nav.LocalName() {
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
				if matches[1] != nav.LocalName() {
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
				if matches[1] != nav.LocalName() {
					return errors.Errorf("%s in attr %s type %s must be the same as node name %s", matches[1], attrName, t, nav.LocalName())
				}
				metaSheet.SetColType(colName, t)
			} else if i == 0 && setKeyedType {
				// case 4: {Type}uint32
				if repeated {
					metaSheet.SetColType(colName, fmt.Sprintf("[%s]<%s>", strcase.ToCamel(nav.LocalName()), t))
				} else {
					metaSheet.SetColType(colName, fmt.Sprintf("{%s}%s", strcase.ToCamel(nav.LocalName()), t))
				}
			} else {
				// default: built-in type
				metaSheet.SetColType(colName, t)
			}
		}
	}

	// iterate over child nodes
	navCopy := *nav
	for flag := navCopy.MoveToChild(); flag; flag = navCopy.MoveToNext() {
		// commentNode, documentNode and other meaningless nodes should be filtered
		if navCopy.NodeType() != xpath.ElementNode {
			continue
		}
		tagName := navCopy.LocalName()
		if err := x.parseNodeType(&navCopy, metaSheet, isMeta); err != nil {
			return errors.Wrapf(err, "failed to parseNodeType for the node %s", tagName)
		}
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

	// add placeholder to nude node
	if len(nav.Current().Attr) == 0 {
		colName := prefix + placeholderName
		// fill values to the bottom when backtrace to top line
		for tmpCusor := cursor; tmpCusor < len(metaSheet.Rows); tmpCusor++ {
			metaSheet.Cell(tmpCusor, len(metaSheet.Rows[metaSheet.Namerow-1].Cells), colName).Data = placeholderValue
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
			if r, exist := prefixMap[reversedList[i]]; exist {
				prefixMap[reversedList[i]] = Range{r.begin, r.attrNum, r.len + 1}
			}
		}
		for k, v := range prefixMap {
			if _, exist := parentMap[k]; !exist && v.begin > r.begin {
				prefixMap[k] = Range{v.begin + 1, v.attrNum, v.len}
			}
		}
	}
	// insert prefixMap
	if r, exist := prefixMap[prefix]; !exist {
		index := len(metaSheet.Rows[metaSheet.Namerow-1].Cells)
		if len(reversedList) > 0 {
			parentPrefix := reversedList[len(reversedList)-1]
			if r2, exist := prefixMap[parentPrefix]; exist {
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
