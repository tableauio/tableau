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
	"github.com/emirpasic/gods/sets/treeset"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/xlsxgen"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
)

type XMLImporter struct {
	*book.Book
}

var attrRegexp *regexp.Regexp
var scalarListRegexp *regexp.Regexp
var metasheetRegexp *regexp.Regexp

const (
	xmlProlog             = `<?xml version='1.0' encoding='UTF-8'?>`
	ungreedyPropGroup     = `(\|\{[^\{\}]+\})?`
	atTableauDisplacement = `ATABLEAU`
	metasheetItemBlock    = `(\s+<Item(\s+\S+\s*=\s*"\S+")+\s*/>\s+)*`
	sheetBlock            = `<%v(>(.*\n)*</%v>|\s*/>)`
)

func init() {
	attrRegexp = regexp.MustCompile(`([0-9A-Za-z_]+)="` + types.TypeGroup + ungreedyPropGroup + `"`)
	scalarListRegexp = regexp.MustCompile(`([A-Za-z_]+)([0-9]+)`)
	metasheetRegexp = regexp.MustCompile(fmt.Sprintf(`<!--\s+(<%v(>`+metasheetItemBlock+`</%v>|\s*/>)(.*\n)+)-->`, book.MetasheetName, book.MetasheetName))
}

// TODO: options
func NewXMLImporter(filename string, sheets []string, parser book.SheetParser) (*XMLImporter, error) {
	newBook, err := parseXML(filename, sheets, parser)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse xml:%s", filename)
	}
	if newBook == nil {
		log.Debugf("xml:%s parsed to an empty book", filename)
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

// splitRawXML splits the raw xml into metasheet and content (which is the xml data)
func splitRawXML(rawXML string) (metasheet, content string) {
	matches := matchMetasheet(rawXML)
	if len(matches) < 2 {
		return "", rawXML
	}
	content = strings.ReplaceAll(rawXML, matches[0], "")
	metasheet = xmlProlog + "\n" + escapeAttrs(strings.ReplaceAll(matches[1], book.MetasheetName, atTableauDisplacement))
	return metasheet, content
}

// parseXML parse sheets in the XML file named `filename` and return a book with multiple sheets
// in TABLEAU grammar which can be exported to protobuf by excel parser.
func parseXML(filename string, sheetNames []string, parser book.SheetParser) (*book.Book, error) {
	log.Debugf("xml: %s", filename)
	buf, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	// pre check if exists `@TABLEAU`
	metasheet, content := splitRawXML(string(buf))
	if metasheet == "" {
		log.Debugf("xml:%s no need parse: %s not found", filename, book.MetasheetName)
		return nil, nil
	}

	// The first pass
	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(bookName, filename, parser)
	xmlMeta, err := readXMLFile(metasheet, content, newBook)
	if err != nil {
		return nil, err
	}

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

	// parse meta sheet
	if parser != nil {
		if err := newBook.ParseMeta(); err != nil {
			return nil, errors.WithMessage(err, "failed to parse metasheet")
		}
	}

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
func readXMLFile(metasheet, content string, newBook *book.Book) (*tableaupb.XMLBook, error) {
	xmlMeta := &tableaupb.XMLBook{
		SheetMap: make(map[string]int32),
	}
	// sheetName -> {colName -> val}
	metasheetMap := make(map[string]map[string]string)
	// parse metasheet
	metaRoot, err := xmlquery.Parse(strings.NewReader(metasheet))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse @TABLEAU string: %s", metasheet)
	}
	for n := metaRoot.FirstChild; n != nil; n = n.NextSibling {
		if n.Type != xmlquery.ElementNode {
			continue
		}
		// <@TABLEAU>...</@TABLEAU>
		if n.Data == atTableauDisplacement {
			var sheet *book.Sheet
			if metasheetMap, sheet, err = genMetasheet(n); err != nil {
				return nil, errors.Wrapf(err, "failed to generate metasheet")
			}
			newBook.AddSheet(sheet)
			continue
		}
		sheetName := n.Data
		xmlSheet := getXMLSheet(xmlMeta, sheetName)
		if err := parseMetaNode(n, xmlSheet); err != nil {
			return nil, errors.Wrapf(err, "failed to parseMetaNode for sheet:%s", sheetName)
		}
	}
	
	// strip template sheets
	for sheet, colMap := range metasheetMap {
		if template, ok := colMap["Template"]; !ok || template != "true" {
			continue
		}
		matches := matchSheetBlock(content, sheet)
		if len(matches) == 0 {
			continue
		}
		content = strings.ReplaceAll(content, matches[0], "")
	}

	// parse data content
	dataRoot, err := xmlquery.Parse(strings.NewReader(content))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse @TABLEAU string: %s", content)
	}
	for n := dataRoot.FirstChild; n != nil; n = n.NextSibling {
		if n.Type != xmlquery.ElementNode {
			continue
		}
		sheet, ok := metasheetMap[n.Data]
		// metasheet not empty and sheet not explicitly declared
		if len(metasheetMap) != 0 && !ok {
			log.Debugf("sheet not set in @TABLEAU, skilpped|sheetName:%v", n.Data)
			continue
		}
		if template, ok := sheet["Template"]; ok && template == "true" {
			continue
		}
		sheetName := n.Data
		xmlSheet := getXMLSheet(xmlMeta, sheetName)
		if err := parseDataNode(n, xmlSheet); err != nil {
			return nil, errors.Wrapf(err, "failed to parseDataNode for sheet:%s", sheetName)
		}
	}

	return xmlMeta, nil
}

// genMetasheet generates metasheet according to `
//   <@TABLEAU>
//       <Item Sheet="XXXConf" />
//   </@TABLEAU>`
func genMetasheet(tableauNode *xmlquery.Node) (map[string]map[string]string, *book.Sheet, error) {
	// sheetName -> {colName -> val}
	metasheetMap := make(map[string]map[string]string)
	nameRow := book.MetasheetOptions().Namerow - 1
	dataRow := book.MetasheetOptions().Datarow - 1
	set := treeset.NewWithStringComparator()
	for n := tableauNode.FirstChild; n != nil; n = n.NextSibling {
		if n.Type != xmlquery.ElementNode {
			continue
		}
		sheetMap := make(map[string]string)
		for _, attr := range n.Attr {
			sheetMap[attr.Name.Local] = attr.Value
		}
		sheetMap["Nested"] = "true"
		// param in `config.yaml` may not be one
		sheetMap["Nameline"] = "1"
		sheetMap["Typeline"] = "1"
		sheetName, ok := sheetMap["Sheet"]
		if !ok {
			return metasheetMap, nil, fmt.Errorf("@TABLEAU not specified sheetName by keyword `Sheet`")
		}
		metasheetMap[sheetName] = sheetMap
		for k := range sheetMap {
			set.Add(k)
		}
	}
	rows := make([][]string, len(metasheetMap)+int(dataRow))
	for _, k := range set.Values() {
		rows[nameRow] = append(rows[nameRow], k.(string))
	}
	for _, sheet := range metasheetMap {
		for _, k := range set.Values() {
			if v, ok := sheet[k.(string)]; ok {
				rows[dataRow] = append(rows[dataRow], v)
			} else {
				rows[dataRow] = append(rows[dataRow], "")
			}
		}
		dataRow++
	}
	sheet := book.NewSheet(book.MetasheetName, rows)
	return metasheetMap, sheet, nil
}

// addMetaNodeAttr adds an attribute to MetaNode AttrMap if not exists and otherwise replaces the attribute value
func addMetaNodeAttr(attrMap *tableaupb.XMLNode_AttrMap, name, val string) {
	if idx, ok := attrMap.Map[name]; !ok {
		attrMap.Map[name] = int32(len(attrMap.List))
		attrMap.List = append(attrMap.List, &tableaupb.XMLNode_AttrMap_Attr{
			Name:  name,
			Value: val,
		})
	} else {
		// replace attribute value by metaSheet
		metaAttr := attrMap.List[idx]
		metaAttr.Value = val
	}
}

// addDataNodeAttr adds an attribute to DataNode AttrMap
func addDataNodeAttr(metaMap, dataMap *tableaupb.XMLNode_AttrMap, name, val string) {
	if _, ok := metaMap.Map[name]; !ok {
		metaMap.Map[name] = int32(len(metaMap.List))
		metaMap.List = append(metaMap.List, &tableaupb.XMLNode_AttrMap_Attr{
			Name:  name,
			Value: inferType(val),
		})
	}
	dataMap.Map[name] = int32(len(dataMap.List))
	dataMap.List = append(dataMap.List, &tableaupb.XMLNode_AttrMap_Attr{
		Name:  name,
		Value: val,
	})
}

// hasChild check if xml node has any child element node
func hasChild(n *xmlquery.Node) bool {
	for n := n.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == xmlquery.ElementNode {
			return true
		}
	}
	return false
}

// getTextContent get the text node from xml node
func getTextContent(n *xmlquery.Node) string {
	if hasChild(n) {
		return ""
	}
	for n := n.FirstChild; n != nil; n = n.NextSibling {
		if n.Type == xmlquery.TextNode {
			return strings.TrimSpace(n.Data)
		}
	}
	return ""
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
		addMetaNodeAttr(meta.AttrMap, attrName, t)
	}
	for n := curr.FirstChild; n != nil; n = n.NextSibling {
		if n.Type != xmlquery.ElementNode {
			continue
		}
		// e.g.: <MaxNum>int32</MaxNum>
		if innerText := getTextContent(n); innerText != "" {
			attrName := n.Data
			addMetaNodeAttr(meta.AttrMap, attrName, innerText)
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
		addDataNodeAttr(meta.AttrMap, data.AttrMap, attrName, attr.Value)
	}
	for n := curr.FirstChild; n != nil; n = n.NextSibling {
		if n.Type != xmlquery.ElementNode {
			continue
		}
		// e.g.: <MaxNum>100</MaxNum>
		if innerText := getTextContent(n); innerText != "" {
			attrName := n.Data
			addDataNodeAttr(meta.AttrMap, data.AttrMap, attrName, innerText)
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

func matchMetasheet(s string) []string {
	return metasheetRegexp.FindStringSubmatch(s)
}

func matchSheetBlock(xml, sheetName string) []string {
	sheetRegexp := regexp.MustCompile(fmt.Sprintf(sheetBlock, sheetName, sheetName))
	return sheetRegexp.FindStringSubmatch(xml)
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
		log.Panicf("duplicated path registered in MetaNodeMap|Path:%s", node.Path)
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
				log.Errorf("strconv.Atoi failed|attr:%s|num:%s|err:%s", attr.Name, matches[2], err)
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
// - []int64: {MapConf}[]int64
//
// NOTE: list and keyedlist auto-deduction not supported temporarily
func fixNodeType(xmlSheet *tableaupb.XMLSheet, curr *tableaupb.XMLNode, oriType string) (t string) {
	t = oriType
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
// genSheet generates a `book.Sheet` which can be furtherly processed by `protogen`.
func genSheet(xmlSheet *tableaupb.XMLSheet) (sheet *book.Sheet, err error) {
	sheetName := xmlSheet.Meta.Name
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
	return sheet, nil
}

// genHeaderRows recursively read meta info from `node` and generates the header rows of `metaSheet`, which is a 2-dimensional IR.
func genHeaderRows(node *tableaupb.XMLNode, metaSheet *xlsxgen.MetaSheet, prefix string) error {
	curPrefix := newPrefix(prefix, node.Name, metaSheet.Worksheet)
	for _, attr := range node.AttrMap.List {
		metaSheet.SetColType(curPrefix+attr.Name, attr.Value)
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
		colName := curPrefix + attr.Name
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
	if curNode != sheetName {
		return prefix + curNode
	} else {
		return prefix
	}
}
