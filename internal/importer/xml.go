package importer

import (
	"bufio"
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

type NoNeedParseError struct {
}

func (e *NoNeedParseError) Error() string {
	return fmt.Sprintf("`%s` not found", book.MetasheetName)
}

var attrRegexp *regexp.Regexp
var scalarListRegexp *regexp.Regexp

const (
	xmlProlog         = `<?xml version='1.0' encoding='UTF-8'?>`
	ungreedyPropGroup = `(\|\{[^\{\}]+\})?`
)

func init() {
	attrRegexp = regexp.MustCompile(`([0-9A-Za-z_]+)="` + types.TypeGroup + ungreedyPropGroup + `"`)
	scalarListRegexp = regexp.MustCompile(`([A-Za-z_]+)([0-9]+)`)
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
	atom.Log.Debugf("xml: %s", filename)
	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open %s", filename)
	}
	defer f.Close()
	// pre check if exists `@TABLEAU`
	scanner := bufio.NewScanner(f)
	foundTableau := false
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), book.MetasheetName) {
			foundTableau = true
			break
		}
	}
	if !foundTableau {
		return nil, nil
	}
	root, err := xmlquery.Parse(f)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse xml:%s", filename)
	}

	// The first pass
	xmlMeta, sheetNames, err := readXMLFile(root, sheetNames)
	if err != nil {
		switch e := err.(type) {
		case *NoNeedParseError:
			atom.Log.Debugf("xml:%s no need parse: %s not found", filename, book.MetasheetName)
			return nil, nil
		default:
			return nil, e
		}
	}

	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(bookName, filename, nil)
	for _, xmlSheet := range xmlMeta.SheetList {
		sheetName := xmlSheet.Meta.Name
		// The second pass
		if err := preprocess(xmlSheet, xmlSheet.Meta); err != nil {
			return nil, errors.Wrapf(err, "failed to preprocess for sheet: %s", sheetName)
		}
		// The third pass
		sheet, err := genSheet(xmlSheet)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to genSheet for sheet: %s", sheetName)
		}
		newBook.AddSheet(sheet)
	}
	atom.Log.Debug(sheetNames)

	if len(sheetNames) > 0 {
		newBook.Squeeze(sheetNames)
	}

	return newBook, nil
}

// --------------------------------------------- THE FIRST PASS ------------------------------------ //
// The first pass simply reads xml file with xmlquery, construct a recursively self-described tree 
// structure defined in xml.proto and put it into memory.
//
// readXMLFile read the raw xml rooted at `root`, specify which sheets to parse and return a XMLBook.
func readXMLFile(root *xmlquery.Node, sheetNames []string) (*tableaupb.XMLBook, []string, error) {
	xmlMeta := &tableaupb.XMLBook{
		SheetMap: make(map[string]int32),
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
			metaStr := xmlProlog + escapeAttrs(strings.ReplaceAll(n.Data, book.MetasheetName, ""))
			// atom.Log.Debug(metaStr)
			metaRoot, err := xmlquery.Parse(strings.NewReader(metaStr))
			if err != nil {
				return nil, nil, errors.Wrapf(err, "failed to parse @TABLEAU string: %s", metaStr)
			}
			for n := metaRoot.FirstChild; n != nil; n = n.NextSibling {
				if n.Type != xmlquery.ElementNode {
					continue
				}
				sheetName := n.Data
				xmlSheet := getXMLSheet(xmlMeta, sheetName)
				if err := parseMetaNode(n, xmlSheet); err != nil {
					return nil, nil, errors.Wrapf(err, "failed to parseMetaNode for sheet:%s", sheetName)
				}
				// append if user not specified
				if noSheetByUser {
					sheetNames = append(sheetNames, sheetName)
				}
			}
		case xmlquery.ElementNode:
			sheetName := n.Data
			xmlSheet := getXMLSheet(xmlMeta, sheetName)
			if err := parseDataNode(n, xmlSheet); err != nil {
				return nil, nil, errors.Wrapf(err, "failed to parseDataNode for sheet:%s", sheetName)
			}
		default:
		}
	}
	if !foundMetaSheetName && noSheetByUser {
		return nil, nil, &NoNeedParseError{}
	}

	return xmlMeta, sheetNames, nil
}

// parseMetaNode parse xml node `curr` and construct the meta tree in `xmlSheet`.
func parseMetaNode(curr *xmlquery.Node, xmlSheet *tableaupb.XMLSheet) error {
	_, path := getNodePath(curr)
	meta := xmlSheet.MetaNodeMap[path]
	for _, attr := range curr.Attr {
		attrName := attr.Name.Local
		t := attr.Value
		if len(meta.AttrMap.List) > 0 && isCrossCell(t) {
			return fmt.Errorf("%s=\"%s\" is a complex type, must be the first attribute", attrName, t)
		}
		if idx, ok := meta.AttrMap.Map[attrName]; !ok {
			meta.AttrMap.Map[attrName] = int32(len(meta.AttrMap.List))
			meta.AttrMap.List = append(meta.AttrMap.List, &tableaupb.XMLNode_AttrMap_Attr{
				Name:  attrName,
				Value: t,
			})
		} else {
			// replace attribute value by metaSheet
			metaAttr := meta.AttrMap.List[idx]
			metaAttr.Value = t
		}
	}
	for n := curr.FirstChild; n != nil; n = n.NextSibling {
		if n.Type != xmlquery.ElementNode {
			continue
		}
		childName := n.Data
		if _, ok := meta.ChildMap[childName]; !ok {
			newNode := newNode(childName, meta)
			meta.ChildMap[childName] = &tableaupb.XMLNode_IndexList{
				Indexes: []int32{int32(len(meta.ChildList))},
			}
			meta.ChildList = append(meta.ChildList, newNode)
			registerMetaNode(xmlSheet, newNode)
		}
		if err := parseMetaNode(n, xmlSheet); err != nil {
			return errors.Wrapf(err, "failed to parseMetaNode for %s@%s", childName, meta.Name)
		}
	}
	return nil
}

// parseDataNode parse xml node `curr`, complete the meta tree and fill the data into `xmlSheet`.
func parseDataNode(curr *xmlquery.Node, xmlSheet *tableaupb.XMLSheet) error {
	_, path := getNodePath(curr)
	meta := xmlSheet.MetaNodeMap[path]
	data_nodes := xmlSheet.DataNodeMap[path].Nodes
	data := data_nodes[len(data_nodes)-1]
	for _, attr := range curr.Attr {
		attrName := attr.Name.Local
		t := inferType(attr.Value)
		if _, ok := meta.AttrMap.Map[attrName]; !ok {
			meta.AttrMap.Map[attrName] = int32(len(meta.AttrMap.List))
			meta.AttrMap.List = append(meta.AttrMap.List, &tableaupb.XMLNode_AttrMap_Attr{
				Name:  attrName,
				Value: t,
			})
		}
		data.AttrMap.Map[attrName] = int32(len(data.AttrMap.List))
		data.AttrMap.List = append(data.AttrMap.List, &tableaupb.XMLNode_AttrMap_Attr{
			Name:  attrName,
			Value: attr.Value,
		})
	}
	for n := curr.FirstChild; n != nil; n = n.NextSibling {
		if n.Type != xmlquery.ElementNode {
			continue
		}
		childName := n.Data
		dataChild := newNode(childName, data)
		registerDataNode(xmlSheet, dataChild)
		if _, ok := meta.ChildMap[childName]; !ok {
			newNode := newNode(childName, meta)
			meta.ChildMap[childName] = &tableaupb.XMLNode_IndexList{
				Indexes: []int32{int32(len(meta.ChildList))},
			}
			meta.ChildList = append(meta.ChildList, newNode)
			registerMetaNode(xmlSheet, newNode)
		}
		if err := parseDataNode(n, xmlSheet); err != nil {
			return errors.Wrapf(err, "failed to parseDataNode for %s@%s", childName, meta.Name)
		}
		if list, ok := data.ChildMap[childName]; !ok {
			data.ChildMap[childName] = &tableaupb.XMLNode_IndexList{
				Indexes: []int32{int32(len(data.ChildList))},
			}
		} else {
			list.Indexes = append(list.Indexes, int32(len(data.ChildList)))
		}
		data.ChildList = append(data.ChildList, dataChild)
	}
	return nil
}

// escapeMetaDoc escape characters for all attribute values in the document. e.g.:
//
//  <ServerConf key="map<uint32,ServerConf>" Open="bool">
// 	 ...
//  </ServerConf>
//
// will be converted to
//
//  <ServerConf key="map&lt;uint32,ServerConf&gt;" Open="bool">
// 	 ...
//  </ServerConf>
func escapeAttrs(doc string) string {
	escapedDoc := attrRegexp.ReplaceAllStringFunc(doc, func(s string) string {
		matches := matchAttr(s)
		var typeBuf, propBuf bytes.Buffer
		xml.EscapeText(&typeBuf, []byte(matches[2]))
		xml.EscapeText(&propBuf, []byte(matches[3]))
		return fmt.Sprintf("%s=\"%s%s\"", matches[1], typeBuf.String(), propBuf.String())
	})
	return escapedDoc
}

// getNodePath get the root and the path walking from root to `curr` in the tree.
func getNodePath(curr *xmlquery.Node) (root *xmlquery.Node, path string) {
	path = curr.Data
	for n := curr.Parent; n != nil; n = n.Parent {
		if n.Data == "" {
			root = n
		} else {
			path = fmt.Sprintf("%s/%s", n.Data, path)

		}
	}
	return root, path
}

// inferType infer type from the node value, e.g.: 
// - 4324342: `int32`
// - 4324324324324343243432: `int64`
// - 4535ffdr43t3r: `string`
func inferType(value string) string {
	if _, err := strconv.Atoi(value); err == nil {
		return "int32"
	} else if _, err := strconv.ParseInt(value, 10, 64); err == nil {
		return "int64"
	} else {
		return "string"
	}
}

func matchAttr(s string) []string {
	return attrRegexp.FindStringSubmatch(s)
}

func matchScalarList(s string) []string {
	return scalarListRegexp.FindStringSubmatch(s)
}

func newOrderedAttrMap() *tableaupb.XMLNode_AttrMap {
	return &tableaupb.XMLNode_AttrMap{
		Map: make(map[string]int32),
	}
}

func newNode(nodeName string, parent *tableaupb.XMLNode) *tableaupb.XMLNode {
	node := &tableaupb.XMLNode{
		Name:     nodeName,
		AttrMap:  newOrderedAttrMap(),
		ChildMap: make(map[string]*tableaupb.XMLNode_IndexList),
		Parent:   parent,
	}
	if parent != nil {
		node.Path = fmt.Sprintf("%s/%s", parent.Path, nodeName)
	} else {
		node.Path = nodeName
	}

	return node
}

func registerMetaNode(xmlSheet *tableaupb.XMLSheet, node *tableaupb.XMLNode) {
	if _, ok := xmlSheet.MetaNodeMap[node.Path]; !ok {
		xmlSheet.MetaNodeMap[node.Path] = node
	} else {
		atom.Log.Panicf("duplicated path registered in MetaNodeMap|Path:%s", node.Path)
	}
}

func registerDataNode(xmlSheet *tableaupb.XMLSheet, node *tableaupb.XMLNode) {
	if list, ok := xmlSheet.DataNodeMap[node.Path]; !ok {
		xmlSheet.DataNodeMap[node.Path] = &tableaupb.XMLSheet_NodeList{
			Nodes: []*tableaupb.XMLNode{node},
		}
	} else {
		list.Nodes = append(list.Nodes, node)
	}
}

func newXMLSheet(sheetName string) *tableaupb.XMLSheet {
	return &tableaupb.XMLSheet{
		Meta:        newNode(sheetName, nil),
		Data:        newNode(sheetName, nil),
		MetaNodeMap: make(map[string]*tableaupb.XMLNode),
		DataNodeMap: make(map[string]*tableaupb.XMLSheet_NodeList),
	}
}

func getXMLSheet(xmlMeta *tableaupb.XMLBook, sheetName string) *tableaupb.XMLSheet {
	if idx, ok := xmlMeta.SheetMap[sheetName]; !ok {
		xmlSheet := newXMLSheet(sheetName)
		registerMetaNode(xmlSheet, xmlSheet.Meta)
		registerDataNode(xmlSheet, xmlSheet.Data)
		xmlMeta.SheetMap[sheetName] = int32(len(xmlMeta.SheetList))
		xmlMeta.SheetList = append(xmlMeta.SheetList, xmlSheet)
		return xmlSheet
	} else {
		return xmlMeta.SheetList[idx]
	}
}

// --------------------------------------------- THE SECOND PASS ------------------------------------ //
// The second pass preprocesses the tree structure. In this phase the parser will do some necessary jobs 
// before generating a 2-dimensional sheet, like correctType which make the types of attributes in the 
// nodes meet the requirements of protogen.
//
func preprocess(xmlSheet *tableaupb.XMLSheet, node *tableaupb.XMLNode) error {
	// rearrange attributes
	if err := rearrangeAttrs(node.AttrMap); err != nil {
		return errors.Wrapf(err, "failed to rearrangeAttrs")
	}
	// fix node types when it is the first attribute
	for i, attr := range node.AttrMap.List {
		if i == 0 {
			attr.Value = fixNodeType(xmlSheet, node, attr.Value)
		}
	}

	// recursively preprocess
	for _, child := range node.ChildList {
		if err := preprocess(xmlSheet, child); err != nil {
			return errors.Wrapf(err, "failed to preprocess node:%s", child.Name)
		}
	}
	return nil
}

// rearrangeAttrs change the order of attributes, e.g.: 
// - attributes with cross-type types, such as cross-cell map (list, keyed-list, etc.), 
//   will be placed at the first.
// - simple list like `Param1, Param2, Param3, ...` will be grouped together and 
//   the type of `Param1` will be changed to something like `[]int32`.
func rearrangeAttrs(attrMap *tableaupb.XMLNode_AttrMap) error {
	typeMap := make(map[string]string)
	indexMap := make(map[int]int)
	for i, attr := range attrMap.List {
		mustFirst := isCrossCell(attr.Value)
		if mustFirst {
			swapAttr(attrMap, i, 0)
			typeMap[attr.Name] = attr.Value
			continue
		}
		matches := matchScalarList(attr.Name)
		if len(matches) > 0 && types.IsScalarType(attr.Value) {
			num, err := strconv.Atoi(matches[2])
			if err != nil {
				atom.Log.Errorf("strconv.Atoi failed|attr:%s|num:%s|err:%s", attr.Name, matches[2], err)
				continue
			}
			indexMap[num] = i
		}
	}
	// start with 1, e.g.: Param1, Param2, ...
	for i, dst := 1, len(attrMap.List)-len(indexMap); ; i, dst = i+1, dst+1 {
		index, ok := indexMap[i]
		if !ok {
			break
		}
		if i == 1 {
			attr := attrMap.List[index]
			attr.Value = fmt.Sprintf("[]%s", attr.Value)
		}
		swapAttr(attrMap, index, dst)
	}
	if len(typeMap) > 1 {
		return fmt.Errorf("more than one non-scalar types: %v", typeMap)
	}
	return nil
}

// fixNodeType fix the type of `curr` in the `xmlSheet` based on its `oriType`. e.g.:
// - map<uint32,Weight>: {Test}map<uint32,Weight>
// - int32: {StructConf}{Weight}int32
// - []int64: {MapConf}[]<int64>
func fixNodeType(xmlSheet *tableaupb.XMLSheet, curr *tableaupb.XMLNode, oriType string) (t string) {
	t = oriType
	if types.IsList(t) {
		matches := types.MatchList(t)
		colType := strings.TrimSpace(matches[2])
		if types.IsScalarType(colType) || types.IsEnum(colType) {
			// list in xml must be keyed list
			t = fmt.Sprintf("[%s]<%s>", matches[1], colType)
		}
	}
	// add type prefixes
	for n, c := curr, curr; n != nil && n.Parent != nil; n, c = n.Parent, n {
		if n == curr {
			// curr is cross-cell, parent should not add prefix
			if isCrossCell(oriType) {
				continue
			}
		} else {
			// not the first attr or not the first child, fix ok
			if len(n.AttrMap.List) > 0 || !isFirstChild(c) {
				break
			}
		}
		if isRepeated(xmlSheet, n) {
			if n == curr {
				t = fmt.Sprintf("[%s]<%s>", n.Name, t)
			} else {
				t = fmt.Sprintf("[%s]%s", n.Name, t)
			}
		} else {
			t = fmt.Sprintf("{%s}%s", n.Name, t)
		}
	}
	return t
}

// isRepeated check if `curr` has other sibling nodes with the same name with itself.
func isRepeated(xmlSheet *tableaupb.XMLSheet, curr *tableaupb.XMLNode) bool {
	strList := strings.Split(curr.Path, "/")
	parentPath := strings.Join(strList[:len(strList)-1], "/")
	if nodes, ok := xmlSheet.DataNodeMap[parentPath]; ok {
		for _, n := range nodes.Nodes {
			if indexes, ok := n.ChildMap[curr.Name]; ok && len(indexes.Indexes) > 1 {
				return true
			}
		}
	}
	return false
}

// isCrossCell check if type string `t` is a cross-cell type.
func isCrossCell(t string) bool {
	if types.IsMap(t) { // map case
		matches := types.MatchMap(t)
		valueType := strings.TrimSpace(matches[2])
		return !(types.IsScalarType(valueType) || types.IsEnum(valueType))
	} else if types.IsList(t) { // list case
		matches := types.MatchList(t)
		structType := strings.TrimSpace(matches[1])
		return structType != ""
	} else if types.IsKeyedList(t) { // keyed-list case
		matches := types.MatchKeyedList(t)
		structType := strings.TrimSpace(matches[1])
		return structType != ""
	} else if types.IsStruct(t) { // struct case
		matches := types.MatchStruct(t)
		colType := strings.TrimSpace(matches[2])
		return types.IsScalarType(colType) || types.IsEnum(colType)
	}
	return false
}

// isFirstChild check if `node` is the first child of its parent node.
func isFirstChild(node *tableaupb.XMLNode) bool {
	if node.Parent == nil {
		return false
	}
	return node.Parent.ChildList[0] == node
}

func swapAttr(attrMap *tableaupb.XMLNode_AttrMap, i, j int) {
	attr := attrMap.List[i]
	attrMap.Map[attr.Name] = int32(j)
	attrMap.Map[attrMap.List[j].Name] = int32(i)
	attrMap.List[i] = attrMap.List[j]
	attrMap.List[j] = attr
}

// --------------------------------------------- THE THIRD PASS ------------------------------------ //
// The third pass transforms the recursive tree structure into a 2-dimensional sheet, 
// which can be further processed into protoconf.
//
// genSheet generates a `book.Sheet` which can furtherly processed by `protogen`.
func genSheet(xmlSheet *tableaupb.XMLSheet) (sheet *book.Sheet, err error) {
	sheetName := strcase.ToCamel(xmlSheet.Meta.Name)
	header := options.NewDefault().Input.Proto.Header
	metaSheet := xlsxgen.NewMetaSheet(sheetName, header, false)
	// generate sheet header rows
	if err := genHeaderRows(xmlSheet.Meta, metaSheet, ""); err != nil {
		return nil, errors.Wrapf(err, "failed to genHeaderRows for sheet: %s", sheetName)
	}
	// fill sheet data rows
	if err := fillDataRows(xmlSheet.Data, metaSheet, "", int(metaSheet.Datarow)-1); err != nil {
		return nil, errors.Wrapf(err, "failed to fillDataRows for sheet: %s", sheetName)
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

// genHeaderRows recursively read meta info from `node` and generates the header rows of `metaSheet`, which is a 2-dimensional IR.
func genHeaderRows(node *tableaupb.XMLNode, metaSheet *xlsxgen.MetaSheet, prefix string) error {
	curPrefix := newPrefix(prefix, node.Name, metaSheet.Worksheet)
	for _, attr := range node.AttrMap.List {
		metaSheet.SetColType(curPrefix+strcase.ToCamel(attr.Name), attr.Value)
	}
	for _, child := range node.ChildList {
		if err := genHeaderRows(child, metaSheet, curPrefix); err != nil {
			return errors.Wrapf(err, "failed to genHeaderRows for %s@%s", child.Name, curPrefix)
		}
	}
	return nil
}

// fillDataRows recursively read data from `node` and fill them to the data rows of `metaSheet`, which is a 2-dimensional IR.
func fillDataRows(node *tableaupb.XMLNode, metaSheet *xlsxgen.MetaSheet, prefix string, cursor int) error {
	curPrefix := newPrefix(prefix, node.Name, metaSheet.Worksheet)
	// clear to the bottom, since `metaSheet.NewRow()` will copy all data of all columns to create a new row
	if len(node.ChildList) == 0 {
		for tmpCusor := cursor; tmpCusor < len(metaSheet.Rows); tmpCusor++ {
			metaSheet.ForEachCol(tmpCusor, func(name string, cell *xlsxgen.Cell) error {
				if strings.HasPrefix(name, curPrefix) {
					cell.Data = ""
				}
				return nil
			})
		}
	}
	for _, attr := range node.AttrMap.List {
		colName := curPrefix + strcase.ToCamel(attr.Name)
		// fill values to the bottom when backtrace to top line
		for tmpCusor := cursor; tmpCusor < len(metaSheet.Rows); tmpCusor++ {
			metaSheet.Cell(tmpCusor, len(metaSheet.Rows[metaSheet.Namerow-1].Cells), colName).Data = attr.Value
		}
	}
	// iterate over child nodes
	nodeMap := make(map[string]int)
	for _, child := range node.ChildList {
		tagName := child.Name
		if count, existed := nodeMap[tagName]; existed {
			// duplicate means a list, should expand vertically
			row := metaSheet.NewRow()
			if err := fillDataRows(child, metaSheet, curPrefix, row.Index); err != nil {
				return errors.Wrapf(err, "fillDataRows %dth node %s@%s failed", count+1, tagName, curPrefix)
			}
			nodeMap[tagName]++
		} else {
			if err := fillDataRows(child, metaSheet, curPrefix, cursor); err != nil {
				return errors.Wrapf(err, "fillDataRows 1st node %s@%s failed", tagName, curPrefix)
			}
			nodeMap[tagName] = 1
		}
	}

	return nil
}

func newPrefix(prefix, curNode, sheetName string) string {
	// sheet name should not occur in the prefix
	if strcase.ToCamel(curNode) != sheetName {
		return prefix + strcase.ToCamel(curNode)
	} else {
		return prefix
	}
}
