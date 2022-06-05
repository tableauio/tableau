package importer

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/xlsxgen"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
)

type XMLImporter struct {
	*book.Book
}

var attrRegexp *regexp.Regexp

func init() {
	attrRegexp = regexp.MustCompile(`([0-9A-Za-z_]+)="` + types.PubTypeGroup + `"`)
}

func matchAttr(s string) []string {
	return attrRegexp.FindStringSubmatch(s)
}

func newOrderedAttrMap() *tableaupb.OrderedAttrMap {
	return &tableaupb.OrderedAttrMap{
		Map: make(map[string]int32),
	}
}

func newMetaProp(nodeName string) *tableaupb.MetaProp {
	return &tableaupb.MetaProp{
		Name:     nodeName,
		AttrMap:  newOrderedAttrMap(),
		ChildMap: make(map[string]int32),
	}
}

func newDataProp(nodeName string) *tableaupb.DataProp {
	return &tableaupb.DataProp{
		Name:    nodeName,
		AttrMap: newOrderedAttrMap(),
	}
}

func newSheetProp(sheetName string) *tableaupb.SheetProp {
	return &tableaupb.SheetProp{
		Meta: newMetaProp(sheetName),
		Data: newDataProp(sheetName),
	}
}

func getSheetProp(xmlProp *tableaupb.XMLProp, sheetName string) *tableaupb.SheetProp {
	if _, ok := xmlProp.SheetPropMap[sheetName]; !ok {
		xmlProp.SheetPropMap[sheetName] = newSheetProp(sheetName)
	}
	return xmlProp.SheetPropMap[sheetName]
}

// escapeMetaDoc escape characters for all attribute values in the document. e.g.:
//
// <ServerConf key="map<uint32,ServerConf>" Open="bool">
// 	...
// </ServerConf>
//
// will be converted to
//
// <ServerConf key="map&lt;uint32,ServerConf&gt;" Open="bool">
// 	...
// </ServerConf>
func escapeAttrs(doc string) string {
	escapedDoc := attrRegexp.ReplaceAllStringFunc(doc, func(s string) string {
		matches := matchAttr(s)
		var buf bytes.Buffer
		xml.EscapeText(&buf, []byte(matches[2]))
		return fmt.Sprintf("%s=\"%s\"", matches[1], buf.String())
	})
	return escapedDoc
}

func isFirstChild(curr *xmlquery.Node) bool {
	p := curr.Parent
	if p == nil {
		return false
	}
	for n := p.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == xmlquery.ElementNode {
			return n == curr
		}
	}

	return false
}

func isRepeated(root, curr *xmlquery.Node) bool {
	parentPath := ""
	// curr is a sheet node
	if curr.Parent == nil || curr.Parent.Data == "" {
		return false
	}
	for n := curr.Parent; n != nil; n = n.Parent {
		if parentPath == "" {
			parentPath = n.Data
		} else {
			parentPath = fmt.Sprintf("%s/%s", n.Data, parentPath)
		}
	}
	for _, n := range xmlquery.Find(root, parentPath) {
		if len(xmlquery.Find(n, curr.Data)) > 1 {
			return true
		}
	}
	return false
}

func isComplexType(t string) bool {
	return types.IsMap(t) || types.IsList(t) || types.IsKeyedList(t) || types.IsStruct(t)
}

func correctType(root, curr *xmlquery.Node, oriType string) (t string) {
	if !isComplexType(oriType) && (curr.Parent != nil && curr.Parent.Data != "") {
		if isRepeated(root, curr) {
			t = fmt.Sprintf("[%s]<%s>", curr.Data, oriType)
		} else {
			t = fmt.Sprintf("{%s}%s", curr.Data, oriType)
		}
	} else {
		t = oriType
	}
	for n := curr.Parent; n != nil && (n.Parent != nil && n.Parent.Data != ""); n = n.Parent {
		if len(n.Attr) > 0 {
			break
		}
		if isRepeated(root, n) {
			t = fmt.Sprintf("[%s]%s", n.Data, t)
		} else {
			t = fmt.Sprintf("{%s}%s", n.Data, t)
		}
	}
	return t
}

func rearrangeAttrs(attrMap *tableaupb.OrderedAttrMap) error {
	typeMap := make(map[string]string)
	for i, attr := range attrMap.List {
		mustFirst := isComplexType(attr.Value)
		if mustFirst {
			attrMap.Map[attr.Name] = 0
			attrMap.Map[attrMap.List[0].Name] = int32(i)
			attrMap.List[i] = attrMap.List[0]
			attrMap.List[0] = attr
			typeMap[attr.Name] = attr.Value
			continue
		}
	}
	if len(typeMap) > 1 {
		return fmt.Errorf("more than one non-scalar types: %v", typeMap)
	}
	return nil
}

func inferType(value string) string {
	if _, err := strconv.Atoi(value); err == nil {
		return "int32"
	} else if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return "int64"
	} else {
		return "string"
	}
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

func parseMetaNode(root, curr *xmlquery.Node, meta *tableaupb.MetaProp) error {
	for _, attr := range curr.Attr {
		attrName := attr.Name.Local
		t := attr.Value
		if len(meta.AttrMap.List) == 0 {
			t = correctType(root, curr, t)
		} else if isComplexType(t) {
			return fmt.Errorf("%s=\"%s\" is a complex type, must be the first attribute", attrName, t)
		}
		if idx, ok := meta.AttrMap.Map[attrName]; !ok {
			meta.AttrMap.Map[attrName] = int32(len(meta.AttrMap.List))
			meta.AttrMap.List = append(meta.AttrMap.List, &tableaupb.Attr{
				Name:  attrName,
				Value: t,
			})
		} else {
			// replace attribute value by metaSheet
			propAttr := meta.AttrMap.List[idx]
			propAttr.Value = t
		}
	}
	for n := curr.FirstChild; n != nil; n = n.NextSibling {
		if n.Type != xmlquery.ElementNode {
			continue
		}
		childName := n.Data
		if idx, ok := meta.ChildMap[childName]; !ok {
			meta.ChildMap[childName] = int32(len(meta.ChildList))
			meta.ChildList = append(meta.ChildList, newMetaProp(childName))
			if err := parseMetaNode(root, n, meta.ChildList[len(meta.ChildList)-1]); err != nil {
				return errors.Wrapf(err, "failed to parseMetaNode for %s@%s", childName, meta.Name)
			}
		} else {
			child := meta.ChildList[idx]
			if err := parseMetaNode(root, n, child); err != nil {
				return errors.Wrapf(err, "failed to parseMetaNode for %s@%s", childName, meta.Name)
			}
		}
	}
	return nil
}

func parseDataNode(root, curr *xmlquery.Node, meta *tableaupb.MetaProp, data *tableaupb.DataProp) error {
	for _, attr := range curr.Attr {
		attrName := attr.Name.Local
		t := inferType(attr.Value)
		// correct types
		if len(meta.AttrMap.List) == 0 {
			t = correctType(root, curr, t)
		}
		if _, ok := meta.AttrMap.Map[attrName]; !ok {
			meta.AttrMap.Map[attrName] = int32(len(meta.AttrMap.List))
			meta.AttrMap.List = append(meta.AttrMap.List, &tableaupb.Attr{
				Name:  attrName,
				Value: t,
			})
		}
		data.AttrMap.Map[attrName] = int32(len(data.AttrMap.List))
		data.AttrMap.List = append(data.AttrMap.List, &tableaupb.Attr{
			Name:  attrName,
			Value: attr.Value,
		})
	}
	for n := curr.FirstChild; n != nil; n = n.NextSibling {
		if n.Type != xmlquery.ElementNode {
			continue
		}
		childName := n.Data
		dataChild := newDataProp(childName)
		if idx, ok := meta.ChildMap[childName]; !ok {
			meta.ChildMap[childName] = int32(len(meta.ChildList))
			meta.ChildList = append(meta.ChildList, newMetaProp(childName))
			if err := parseDataNode(root, n, meta.ChildList[len(meta.ChildList)-1], dataChild); err != nil {
				return errors.Wrapf(err, "failed to parseDataNode for %s@%s", childName, meta.Name)
			}
		} else {
			metaChild := meta.ChildList[idx]
			if err := parseDataNode(root, n, metaChild, dataChild); err != nil {
				return errors.Wrapf(err, "failed to parseDataNode for %s@%s", childName, meta.Name)
			}
		}
		data.ChildList = append(data.ChildList, dataChild)
	}
	return nil
}

func genSheetHeaderRows(metaProp *tableaupb.MetaProp, metaSheet *xlsxgen.MetaSheet, prefix string) error {
	curPrefix := prefix
	// sheet name should not occur in the prefix
	if strcase.ToCamel(metaProp.Name) != metaSheet.Worksheet {
		curPrefix = prefix + strcase.ToCamel(metaProp.Name)
	}
	if err := rearrangeAttrs(metaProp.AttrMap); err != nil {
		return errors.Wrapf(err, "failed to rearrangeAttrs")
	}
	for _, attr := range metaProp.AttrMap.List {
		metaSheet.SetColType(curPrefix+strcase.ToCamel(attr.Name), attr.Value)
	}
	for _, child := range metaProp.ChildList {
		if err := genSheetHeaderRows(child, metaSheet, curPrefix); err != nil {
			return errors.Wrapf(err, "failed to genSheetHeaderRows for %s@%s", child.Name, curPrefix)
		}
	}
	return nil
}

func fillSheetDataRows(dataProp *tableaupb.DataProp, metaSheet *xlsxgen.MetaSheet, prefix string, cursor int) error {
	curPrefix := prefix
	// sheet name should not occur in the prefix
	if strcase.ToCamel(dataProp.Name) != metaSheet.Worksheet {
		curPrefix = prefix + strcase.ToCamel(dataProp.Name)
	}
	// clear to the bottom, since `metaSheet.NewRow()` will copy all data of all columns to create a new row
	if len(dataProp.ChildList) == 0 {
		for tmpCusor := cursor; tmpCusor < len(metaSheet.Rows); tmpCusor++ {
			metaSheet.ForEachCol(tmpCusor, func(name string, cell *xlsxgen.Cell) error {
				if strings.HasPrefix(name, curPrefix) {
					cell.Data = ""
				}
				return nil
			})
		}
	}
	for _, attr := range dataProp.AttrMap.List {
		colName := curPrefix + strcase.ToCamel(attr.Name)
		// fill values to the bottom when backtrace to top line
		for tmpCusor := cursor; tmpCusor < len(metaSheet.Rows); tmpCusor++ {
			metaSheet.Cell(tmpCusor, len(metaSheet.Rows[metaSheet.Namerow-1].Cells), colName).Data = attr.Value
		}
	}
	// iterate over child nodes
	nodeMap := make(map[string]int)
	for _, child := range dataProp.ChildList {
		tagName := child.Name
		if count, existed := nodeMap[tagName]; existed {
			// duplicate means a list, should expand vertically
			row := metaSheet.NewRow()
			if err := fillSheetDataRows(child, metaSheet, curPrefix, row.Index); err != nil {
				return errors.Wrapf(err, "fillSheetDataRows %dth node %s@%s failed", count+1, tagName, curPrefix)
			}
			nodeMap[tagName]++
		} else {
			if err := fillSheetDataRows(child, metaSheet, curPrefix, cursor); err != nil {
				return errors.Wrapf(err, "fillSheetDataRows 1st node %s@%s failed", tagName, curPrefix)
			}
			nodeMap[tagName] = 1
		}
	}

	return nil
}

func genSheet(sheetProp *tableaupb.SheetProp) (sheet *book.Sheet, err error) {
	sheetName := strcase.ToCamel(sheetProp.Meta.Name)
	header := options.NewDefault().Input.Proto.Header
	metaSheet := xlsxgen.NewMetaSheet(sheetName, header, false)
	// generate sheet header rows
	if err := genSheetHeaderRows(sheetProp.Meta, metaSheet, ""); err != nil {
		return nil, errors.Wrapf(err, "failed to genSheetHeaderRows for sheet: %s", sheetName)
	}
	// fill sheet data rows
	if err := fillSheetDataRows(sheetProp.Data, metaSheet, "", int(metaSheet.Datarow)-1); err != nil {
		return nil, errors.Wrapf(err, "failed to fillSheetDataRows for sheet: %s", sheetName)
	}
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
	sheet = book.NewSheet(sheetName, rows)
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
	return sheet, nil
}

// parseXML parse sheets in the XML file named `filename` and return a book with multiple sheets
// in TABLEAU grammar which can be exported to protobuf by excel parser.
func parseXML(filename string, sheetNames []string) (*book.Book, error) {
	// open xml file and parse the document
	atom.Log.Debugf("xml: %s", filename)
	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %s", filename)
	}

	root, err := xmlquery.Parse(f)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse xml:%s", filename)
	}
	xmlProp := &tableaupb.XMLProp{
		SheetPropMap: make(map[string]*tableaupb.SheetProp),
	}
	noSheetByUser := len(sheetNames) == 0
	foundMetaSheetName := false
	for n := root.FirstChild; n != nil; n = n.NextSibling {
		switch n.Type {
		case xmlquery.CommentNode:
			if !strings.Contains(n.Data, book.MetasheetName) {
				continue
			}
			foundMetaSheetName = true
			metaStr := escapeAttrs(strings.ReplaceAll(n.Data, book.MetasheetName, ""))
			atom.Log.Debug(metaStr)
			metaRoot, err := xmlquery.Parse(strings.NewReader(metaStr))
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse @TABLEAU string: %s", metaStr)
			}
			for n := metaRoot.FirstChild; n != nil; n = n.NextSibling {
				if n.Type != xmlquery.ElementNode {
					continue
				}
				sheetName := n.Data
				sheetProp := getSheetProp(xmlProp, sheetName)
				if err := parseMetaNode(metaRoot, n, sheetProp.Meta); err != nil {
					return nil, errors.Wrapf(err, "failed to parseMetaNode for sheet:%s", sheetName)
				}
				// append if user not specified
				if noSheetByUser {
					sheetNames = append(sheetNames, sheetName)
				}
			}
		case xmlquery.ElementNode:
			sheetName := n.Data
			sheetProp := getSheetProp(xmlProp, sheetName)
			if err := parseDataNode(root, n, sheetProp.Meta, sheetProp.Data); err != nil {
				return nil, errors.Wrapf(err, "failed to parseDataNode for sheet:%s", sheetName)
			}
		default:
		}
	}
	if !foundMetaSheetName {
		atom.Log.Debugf("xml:%s no need parse: @TABLEAU not found", filename)
		return nil, nil
	}
	atom.Log.Debugf("%v\n", xmlProp)

	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(bookName, filename, nil)
	for _, sheetProp := range xmlProp.SheetPropMap {
		sheet, err := genSheet(sheetProp)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to genSheet for sheet: %s", sheetProp.Meta.Name)
		}
		newBook.AddSheet(sheet)
	}
	atom.Log.Debug(sheetNames)

	if len(sheetNames) > 0 {
		newBook.Squeeze(sheetNames)
	}

	return newBook, nil
}
