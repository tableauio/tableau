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
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
)

type XMLImporter struct {
	*book.Book
}

var attrRegexp *regexp.Regexp
var tagRegexp *regexp.Regexp
var scalarListRegexp *regexp.Regexp
var metasheetRegexp *regexp.Regexp

const (
	xmlProlog             = `<?xml version='1.0' encoding='UTF-8'?>`
	atTableauDisplacement = `ATABLEAU`
	ungreedyPropGroup     = `(\|\{[^\{\}]+\})?`                       // e.g.: |{default:"100"}
	metasheetItemBlock    = `<Item(\s+\S+\s*=\s*("\S+"|'\S+'))+\s*/>` // e.g.: <Item Sheet="XXXConf" Sep="|"/>
	sheetBlock            = `<%v(>(.*\n)*</%v>|\s*/>)`                // e.g.: <XXXConf>...</XXXConf>
)

func init() {
	attrRegexp = regexp.MustCompile(`\s*=\s*("|')` + types.TypeGroup + ungreedyPropGroup + `("|')`) // e.g.: = "int32|{range:"1,~"}"
	tagRegexp = regexp.MustCompile(`>` + types.TypeGroup + ungreedyPropGroup + `</`)                // e.g.: >int32|{range:"1,~"}</
	scalarListRegexp = regexp.MustCompile(`([A-Za-z_]+)([0-9]+)`)                                   // e.g.: Para1, Para2, Para3, ...

	// metasheet regexp, e.g.:
	// <!--
	// <@TABLEAU>
	// 		<Item Sheet="Server" />
	// </@TABLEAU>

	// <Server>
	// 		<Weight Num="map<uint32, Weight>"/>
	// </Server>
	// -->
	metasheetRegexp = regexp.MustCompile(fmt.Sprintf(`<!--\s+(<%v(>(\s+`+metasheetItemBlock+`\s+)*</%v>|\s*/>)(.*\n)+?)-->\s*\n`, book.MetasheetName, book.MetasheetName))
}

// TODO: options
func NewXMLImporter(filename string, sheets []string, parser book.SheetParser, mode ImporterMode, cloned bool, primaryBookName string) (*XMLImporter, error) {
	newBook, err := parseXML(filename, sheets, parser, mode, cloned, primaryBookName)
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

	// log.Debugf("book: %+v", newBook)

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
	scanner := bufio.NewScanner(strings.NewReader(matches[0]))
	emptyLines := ""
	for scanner.Scan() {
		emptyLines += "\n"
	}
	if err := scanner.Err(); err != nil {
		log.Panicf("scanner err:%v", err)
		return "", rawXML
	}
	content = strings.ReplaceAll(rawXML, matches[0], emptyLines)
	metasheet = xmlProlog + "\n" + escapeAttrs(strings.ReplaceAll(matches[1], book.MetasheetName, atTableauDisplacement))
	return metasheet, content
}

// parseXML parse sheets in the XML file named `filename` and return a book with multiple sheets
// in TABLEAU grammar which can be exported to protobuf by excel parser.
func parseXML(filename string, sheetNames []string, parser book.SheetParser, mode ImporterMode, cloned bool, primaryBookName string) (*book.Book, error) {
	log.Debugf("xml:%s|cloned:%v|primaryBookName:%s", filename, cloned, primaryBookName)
	buf, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	metasheet, content := splitRawXML(string(buf))
	// use primary book's metasheet if cloned
	if cloned {
		primaryBookBuf, err := os.ReadFile(primaryBookName)
		if err != nil {
			return nil, err
		}
		metasheet, _ = splitRawXML(string(primaryBookBuf))
	}

	// The first pass
	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(bookName, filename, parser)
	xmlMeta, err := readXMLFile(metasheet, content, newBook, mode)
	if err != nil {
		return nil, err
	}

	for _, xmlSheet := range xmlMeta.SheetList {
		sheetName := xmlSheet.Meta.Name
		// The second pass
		if err := preprocessMeta(xmlSheet, xmlSheet.Meta); err != nil {
			return nil, errors.Wrapf(err, "failed to preprocessMeta for sheet: %s", sheetName)
		}
		if err := preprocessData(xmlSheet, xmlSheet.Data); err != nil {
			return nil, errors.Wrapf(err, "failed to preprocessData for sheet: %s", sheetName)
		}
		// The third pass
		metaSheet, dataSheet, err := genSheet(xmlSheet)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to genSheet for sheet: %s", sheetName)
		}
		newBook.AddSheet(metaSheet)
		newBook.AddSheet(dataSheet)
	}

	// parse meta sheet
	if parser != nil {
		if err := newBook.ParseMetaAndPurge(); err != nil {
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
func readXMLFile(metasheet, content string, newBook *book.Book, mode ImporterMode) (*tableaupb.XMLBook, error) {
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

	if mode == Protogen {
		// strip template sheets
		for sheetname, colMap := range metasheetMap {
			if template, ok := colMap["Template"]; !ok || template != "true" {
				continue
			}
			matches := matchSheetBlock(content, sheetname)
			if len(matches) == 0 {
				continue
			}
			content = strings.ReplaceAll(content, matches[0], "")
		}
	}

	// parse data content
	dataRoot, err := xmlquery.Parse(strings.NewReader(content))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse XML: %s", content)
	}
	for n := dataRoot.FirstChild; n != nil; n = n.NextSibling {
		if n.Type != xmlquery.ElementNode {
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

// genMetasheet generates metasheet according to
//
//	<@TABLEAU>
//		<Item Sheet="XXXConf" />
//	</@TABLEAU>
func genMetasheet(tableauNode *xmlquery.Node) (map[string]map[string]string, *book.Sheet, error) {
	root := &book.Node{
		Kind: book.MapNode,
		Children: []*book.Node{
			{
				Kind:  book.MapNode,
				Name:  book.KeywordSheet,
				Value: book.MetasheetName,
			},
		},
	}
	// sheetName -> {colName -> val}
	metasheetMap := make(map[string]map[string]string)
	for n := tableauNode.FirstChild; n != nil; n = n.NextSibling {
		if n.Type != xmlquery.ElementNode {
			continue
		}
		var children []*book.Node
		sheetMap := make(map[string]string)
		var sheetName string
		for _, attr := range n.Attr {
			name := attr.Name.Local
			value := attr.Value
			children = append(children, &book.Node{
				Name:  name,
				Value: value,
			})
			sheetMap[name] = value

			if name == "Sheet" {
				sheetName = value
			}
		}
		if sheetName == "" {
			return metasheetMap, nil, errors.Errorf("field `Sheet` not specified in metasheet @TABLEAU")
		}
		metasheetMap[sheetName] = sheetMap
		sheetNode := &book.Node{
			Kind:     book.MapNode,
			Name:     sheetName,
			Children: children,
		}
		root.Children = append(root.Children, sheetNode)
	}
	doc := &book.Node{
		Kind:     book.DocumentNode,
		Name:     book.MetasheetName,
		Children: []*book.Node{root},
	}
	sheet := book.NewDocumentSheet(book.MetasheetName, doc)
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
func addDataNodeAttr(dataMap *tableaupb.XMLNode_AttrMap, name, val string) {
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
		addDataNodeAttr(data.AttrMap, attrName, attr.Value)
	}
	for n := curr.FirstChild; n != nil; n = n.NextSibling {
		if n.Type != xmlquery.ElementNode {
			continue
		}
		// e.g.: <MaxNum>100</MaxNum>
		if innerText := getTextContent(n); innerText != "" {
			attrName := n.Data
			addDataNodeAttr(data.AttrMap, attrName, innerText)
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
//	 <ServerConf key="map<uint32,ServerConf>" Open="bool">
//		 ...
//	 </ServerConf>
//
// will be converted to
//
//	 <ServerConf key="map&lt;uint32,ServerConf&gt;" Open="bool">
//		 ...
//	 </ServerConf>
func escapeAttrs(doc string) string {
	escapedDoc := attrRegexp.ReplaceAllStringFunc(doc, func(s string) string {
		matches := matchAttr(s)
		var typeBuf, propBuf bytes.Buffer
		xml.EscapeText(&typeBuf, []byte(matches[2]))
		xml.EscapeText(&propBuf, []byte(matches[3]))
		return fmt.Sprintf("=\"%s%s\"", typeBuf.String(), propBuf.String())
	})
	escapedDoc = tagRegexp.ReplaceAllStringFunc(escapedDoc, func(s string) string {
		matches := matchTag(s)
		var typeBuf, propBuf bytes.Buffer
		xml.EscapeText(&typeBuf, []byte(matches[1]))
		xml.EscapeText(&propBuf, []byte(matches[2]))
		return fmt.Sprintf(">%s%s</", typeBuf.String(), propBuf.String())
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

func matchAttr(s string) []string {
	return attrRegexp.FindStringSubmatch(s)
}

func matchTag(s string) []string {
	return tagRegexp.FindStringSubmatch(s)
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
func preprocessMeta(xmlSheet *tableaupb.XMLSheet, node *tableaupb.XMLNode) error {
	// rearrange attributes
	if err := rearrangeAttrs(node.AttrMap); err != nil {
		return errors.Wrapf(err, "failed to rearrangeAttrs")
	}
	// fix node types when it is the first attribute
	// for i, attr := range node.AttrMap.List {
	// 	if i == 0 {
	// 		attr.Value = fixNodeType(xmlSheet, node, attr.Value)
	// 	}
	// }

	// recursively preprocessMeta
	for _, child := range node.ChildList {
		if err := preprocessMeta(xmlSheet, child); err != nil {
			return errors.Wrapf(err, "failed to preprocessMeta node:%s", child.Name)
		}
	}
	return nil
}

func preprocessData(xmlSheet *tableaupb.XMLSheet, node *tableaupb.XMLNode) error {
	path := node.Path
	// read type info from meta node map
	meta, ok := xmlSheet.MetaNodeMap[path]
	if !ok {
		return errors.Errorf("Node[%s] has no meta definition", path)
	}
	for _, attr := range meta.AttrMap.List {
		mustFirst := isCrossCell(attr.Value)
		if mustFirst {
			swapAttr(node.AttrMap, int(node.AttrMap.Map[attr.Name]), 0)
			break
		}
	}

	// recursively preprocessData
	for _, child := range node.ChildList {
		if err := preprocessData(xmlSheet, child); err != nil {
			return errors.Wrapf(err, "failed to preprocessData node:%s", child.Name)
		}
	}
	return nil
}

// rearrangeAttrs change the order of attributes, e.g.:
//   - attributes with cross-type types, such as cross-cell map (list, keyed-list, etc.),
//     will be placed at the first.
//   - simple list like `Param1, Param2, Param3, ...` will be grouped together and
//     the type of `Param1` will be changed to something like `[]int32`.
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
		desc := types.MatchMap(t)
		return !(types.IsScalarType(desc.ValueType) || types.IsEnum(desc.ValueType))
	} else if types.IsList(t) { // list case
		desc := types.MatchList(t)
		return desc.ElemType != ""
	} else if types.IsKeyedList(t) { // keyed-list case
		desc := types.MatchKeyedList(t)
		return desc.ElemType != ""
	} else if types.IsStruct(t) { // struct case
		desc := types.MatchStruct(t)
		return types.IsScalarType(desc.ColumnType) || types.IsEnum(desc.ColumnType)
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
func genSheet(xmlSheet *tableaupb.XMLSheet) (metaSheet *book.Sheet, dataSheet *book.Sheet, err error) {
	sheetName := fmt.Sprintf("@%s", xmlSheet.Meta.Name)
	root := &book.Node{Name: sheetName}
	// fill meta nodes
	if err := fillMetaNode(xmlSheet, xmlSheet.Meta, root); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to fillMetaNode for sheet: %s", sheetName)
	}
	metaSheet = book.NewDocumentSheet(sheetName, &book.Node{
		Kind:     book.DocumentNode,
		Name:     sheetName,
		Children: []*book.Node{root},
	})

	sheetName = xmlSheet.Meta.Name
	root = &book.Node{Name: sheetName}
	// fill data nodes
	if err := fillDataNode(xmlSheet, xmlSheet.Data, root); err != nil {
		return nil, nil, errors.Wrapf(err, "failed to fillDataNode for sheet: %s", sheetName)
	}
	dataSheet = book.NewDocumentSheet(sheetName, &book.Node{
		Kind:     book.DocumentNode,
		Name:     sheetName,
		Children: []*book.Node{root},
	})
	return metaSheet, dataSheet, nil
}

// fillMetaNode recursively read data from `node` and fill them to the data rows of `metaSheet`.
func fillMetaNode(xmlSheet *tableaupb.XMLSheet, node *tableaupb.XMLNode, bnode *book.Node) error {
	path := node.Path
	meta, ok := xmlSheet.MetaNodeMap[path]
	if !ok {
		return errors.Errorf("Node[%s] has no meta definition", path)
	}
	bnode.Kind = book.MapNode
	if meta.Parent == nil {
		// generate `@sheet: TableName` when level = 0
		bnode.Children = append(bnode.Children, &book.Node{
			Name:  book.KeywordSheet,
			Value: bnode.Name,
		})
	}
	// NOTE: curBNode may be pointed to one subnode when needed
	curBNode := bnode
	for i, attr := range node.AttrMap.List {
		if desc := types.MatchMap(attr.Value); desc != nil {
			curBNode = &book.Node{
				Kind: book.MapNode,
				Name: book.KeywordStruct,
				Children: []*book.Node{
					{
						Name:  book.KeywordKey,
						Value: attr.Name,
					},
					{
						Name:  book.KeywordKeyname,
						Value: attr.Name,
					},
				},
			}
			bnode.Children = append(bnode.Children, &book.Node{
				Name:  book.KeywordType,
				Value: attr.Value,
			}, &book.Node{
				Name:  book.KeywordVariable,
				Value: fmt.Sprintf("%sMap", node.Name),
			}, curBNode)
		} else if desc := types.MatchKeyedList(attr.Value); desc != nil {
			// treat keyed list as normal list
			curBNode = &book.Node{
				Kind: book.MapNode,
				Name: book.KeywordStruct,
				Children: []*book.Node{
					{
						Name:  book.KeywordKey,
						Value: attr.Name,
					},
					{
						Name:  attr.Name,
						Value: desc.ColumnType,
					},
				},
			}
			bnode.Children = append(bnode.Children, &book.Node{
				Name:  book.KeywordType,
				Value: fmt.Sprintf("[%s]", node.Name),
			}, &book.Node{
				Name:  book.KeywordVariable,
				Value: fmt.Sprintf("%sList", node.Name),
			}, curBNode)
		} else if desc := types.MatchList(attr.Value); desc != nil {
			if desc.ElemType != "" {
				// struct list
				curBNode = &book.Node{
					Kind: book.MapNode,
					Name: book.KeywordStruct,
					Children: []*book.Node{
						{
							Name:  attr.Name,
							Value: desc.ColumnType,
						},
					},
				}
				bnode.Children = append(bnode.Children, &book.Node{
					Name:  book.KeywordType,
					Value: fmt.Sprintf("[%s]", node.Name),
				}, &book.Node{
					Name:  book.KeywordVariable,
					Value: fmt.Sprintf("%sList", node.Name),
				}, curBNode)
			} else {
				// incell list
				curBNode = &book.Node{
					Kind: book.MapNode,
					Name: book.KeywordStruct,
					Children: []*book.Node{
						{
							Kind: book.MapNode,
							Name: attr.Name,
							Children: []*book.Node{
								{
									Name:  book.KeywordType,
									Value: fmt.Sprintf("[%s]", desc.ColumnType),
								},
								{
									Name:  book.KeywordVariable,
									Value: fmt.Sprintf("%sList", attr.Name),
								},
								{
									Name:  book.KeywordIncell,
									Value: "true",
								},
							},
						},
					},
				}
				bnode.Children = append(bnode.Children, &book.Node{
					Name:  book.KeywordType,
					Value: fmt.Sprintf("{%s}", node.Name),
				}, curBNode)
			}
		} else if desc := types.MatchStruct(attr.Value); desc != nil {
			curBNode = &book.Node{
				Kind: book.MapNode,
				Name: book.KeywordStruct,
				Children: []*book.Node{
					{
						Name:  attr.Name,
						Value: desc.ColumnType,
					},
				},
			}
			bnode.Children = append(bnode.Children, &book.Node{
				Name:  book.KeywordType,
				Value: fmt.Sprintf("{%s}", node.Name),
			}, curBNode)
		} else if i == 0 && meta.Parent != nil {
			// generate struct when first encounter scalar attribute
			curBNode = &book.Node{
				Kind: book.MapNode,
				Name: book.KeywordStruct,
				Children: []*book.Node{
					{
						Name:  attr.Name,
						Value: attr.Value,
					},
				},
			}
			bnode.Children = append(bnode.Children, &book.Node{
				Name:  book.KeywordType,
				Value: fmt.Sprintf("{%s}", node.Name),
			}, curBNode)
		} else {
			curBNode.Children = append(curBNode.Children, &book.Node{
				Name:  attr.Name,
				Value: attr.Value,
			})
		}
	}
	// generate struct even if encounter empty node (level>=1)
	if meta.Parent != nil && len(node.AttrMap.List) == 0 {
		curBNode = &book.Node{
			Kind: book.MapNode,
			Name: book.KeywordStruct,
		}
		bnode.Children = append(bnode.Children, &book.Node{
			Name:  book.KeywordType,
			Value: fmt.Sprintf("{%s}", node.Name),
		}, curBNode)
	}
	// iterate over child nodes
	nodeMap := make(map[string]*book.Node)
	for _, child := range node.ChildList {
		tagName := child.Name
		if subNode, existed := nodeMap[tagName]; existed {
			if err := fillMetaNode(xmlSheet, child, subNode); err != nil {
				return errors.Wrapf(err, "failed to fillMetaNode for %s@%s", tagName, node.Name)
			}
		} else {
			subNode := &book.Node{
				Name: tagName,
			}
			curBNode.Children = append(curBNode.Children, subNode)
			if err := fillMetaNode(xmlSheet, child, subNode); err != nil {
				return errors.Wrapf(err, "failed to fillMetaNode for %s@%s", tagName, node.Name)
			}
			nodeMap[tagName] = subNode
		}
	}

	return nil
}

// fillDataNode recursively read data from `node` and fill them to the data rows of `metaSheet`.
func fillDataNode(xmlSheet *tableaupb.XMLSheet, node *tableaupb.XMLNode, bnode *book.Node) error {
	path := node.Path
	// read type info from meta node map
	meta, ok := xmlSheet.MetaNodeMap[path]
	if !ok {
		return errors.Errorf("Node[%s] has no meta definition", path)
	}
	bnode.Kind = book.MapNode
	if meta.Parent == nil {
		// generate `@sheet: TableName` when level = 0
		bnode.Children = append(bnode.Children, &book.Node{
			Name:  book.KeywordSheet,
			Value: bnode.Name,
		})
	}
	// NOTE: curBNode may be pointed to one subnode when needed
	curBNode := bnode
	for _, attr := range node.AttrMap.List {
		index, ok := meta.AttrMap.Map[attr.Name]
		if !ok {
			continue
		}
		typeAttr := meta.AttrMap.List[index].Value
		if desc := types.MatchMap(typeAttr); desc != nil {
			curBNode = &book.Node{
				Kind: book.MapNode,
				Name: attr.Value,
				Children: []*book.Node{
					{
						Name:  attr.Name,
						Value: attr.Value,
					},
				},
			}
			bnode.Children = append(bnode.Children, curBNode)
		} else if desc := types.MatchList(typeAttr); desc != nil {
			if desc.ElemType != "" {
				// struct list
				curBNode = &book.Node{
					Kind: book.MapNode,
					Children: []*book.Node{
						{
							Name:  attr.Name,
							Value: attr.Value,
						},
					},
				}
				bnode.Kind = book.ListNode
				bnode.Children = append(bnode.Children, curBNode)
			} else {
				// incell list
				curBNode.Children = append(curBNode.Children, &book.Node{
					Name:  attr.Name,
					Value: attr.Value,
				})
			}
		} else if desc := types.MatchStruct(typeAttr); desc != nil {
			bnode.Children = append(bnode.Children, &book.Node{
				Name:  attr.Name,
				Value: desc.ColumnType,
			})
		} else {
			curBNode.Children = append(curBNode.Children, &book.Node{
				Name:  attr.Name,
				Value: attr.Value,
			})
		}
	}
	// iterate over child nodes
	nodeMap := make(map[string]*book.Node)
	for _, child := range node.ChildList {
		tagName := child.Name
		if subNode, existed := nodeMap[tagName]; existed {
			if err := fillDataNode(xmlSheet, child, subNode); err != nil {
				return errors.Wrapf(err, "failed to fillDataNode for %s@%s", tagName, node.Name)
			}
		} else {
			subNode := &book.Node{
				Name: tagName,
			}
			curBNode.Children = append(curBNode.Children, subNode)
			if err := fillDataNode(xmlSheet, child, subNode); err != nil {
				return errors.Wrapf(err, "failed to fillDataNode for %s@%s", tagName, node.Name)
			}
			nodeMap[tagName] = subNode
		}
	}

	return nil
}
