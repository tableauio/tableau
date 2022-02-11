package importer

import (
	"bytes"
	"encoding/xml"
	"fmt"
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

type XMLImporter struct {
	filename   string
	sheetMap   map[string]*Sheet // sheet name -> sheet
	sheetNames []string
	header     *options.HeaderOption // header settings.
}

// TODO: options
func NewXMLImporter(filename string, header *options.HeaderOption) *XMLImporter {
	return &XMLImporter{
		filename: filename,
		header:   header,
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
	// open xml file and parse the document
	xmlPath := x.filename
	atom.Log.Debugf("xml: %s", xmlPath)
	buf, err := os.ReadFile(xmlPath)
	if err != nil {
		return errors.Wrapf(err, "failed to open %s", xmlPath)
	}
	// replacement for `<` and `>` not allowed in attribute values
	attrValRegexp := regexp.MustCompile(`"\S+"`)
	nudeRegexp := regexp.MustCompile(`<([A-Za-z0-9]+)>`)
	keywordRegexp := regexp.MustCompile(`([</]+)@([A-Z]+)`)
	replacedStr := attrValRegexp.ReplaceAllStringFunc(string(buf), func(s string) string {
		var buf bytes.Buffer
		xml.EscapeText(&buf, []byte(s[1:len(s)-1]))
		return fmt.Sprintf("\"%s\"", buf.String())
	})
	replacedStr = nudeRegexp.ReplaceAllString(replacedStr, `<$1 TableauPlaceholder="0">`)
	replacedStr = keywordRegexp.ReplaceAllString(replacedStr, `$1$2`)
	s := strings.NewReader(replacedStr)
	p, err := xmlquery.CreateStreamParser(s, "/")
	if err != nil {
		return errors.Wrapf(err, "failed to create parser for string %s", s)
	}
	n, err := p.Read()
	if err != nil {
		return errors.Wrapf(err, "failed to read from string %s", s)
	}
	root := xmlquery.CreateXPathNavigator(n)
	x.sheetNames = append(x.sheetNames, root.LocalName())
	// generate data sheet
	metaSheet := xlsxgen.NewMetaSheet(root.LocalName(), x.header, false)
	if err := x.parseNode(root, metaSheet, int(metaSheet.Datarow)-1); err != nil {
		return errors.Wrapf(err, "parseNode for root node %s failed", root.LocalName())
	}
	var rows [][]string
	for i := 0; i < len(metaSheet.Rows); i++ {
		var row []string
		for _, cell := range metaSheet.Rows[i].Cells {
			row = append(row, cell.Data)
		}
		rows = append(rows, row)
	}
	sheet := NewSheet(root.LocalName(), rows)
	sheet.Meta = &tableaupb.SheetMeta{
		Sheet:    root.LocalName(),
		Alias:    root.LocalName(),
		Nameline: 1,
		Typeline: 1,
		Nested:   true,
	}
	x.sheetMap[root.LocalName()] = sheet
	return nil
}

// parseNode parse and convert an xml file to sheet format
func (x *XMLImporter) parseNode(nav *xmlquery.NodeNavigator, metaSheet *xlsxgen.MetaSheet, cursor int) error {
	// preprocess
	realParent, prefix, isMeta := nav.Current().Parent, "", false
	// skip `TABLEAU` to find real parent
	for flag, navCopy := true, *nav; flag && navCopy.LocalName() != metaSheet.Worksheet; flag = navCopy.MoveToParent() {
		if navCopy.LocalName() == "TABLEAU" {
			isMeta = true
		} else {
			prefix = strcase.ToCamel(navCopy.LocalName()) + prefix
		}
		if navCopy.Current().Parent != nil && navCopy.Current().Parent.Data == "TABLEAU" {
			realParent = navCopy.Current().Parent.Parent
		}
	}
	repeated := len(xmlquery.Find(realParent, nav.LocalName())) > 1

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
	for i, attr := range nav.Current().Attr {
		attrName := attr.Name.Local
		attrValue := attr.Value
		t, d := inferType(attrValue)
		colName := prefix + strcase.ToCamel(attrName)
		lastColName := metaSheet.GetLastColName()
		metaSheet.SetDefaultValue(colName, d)
		if isMeta {
			if index := strings.Index(attrValue, "|"); index > 0 {
				t = attrValue[:index]
				metaSheet.SetDefaultValue(colName, attrValue[index+1:])
			} else {
				t = attrValue
			}
		} else {
			// fill values to the bottom when backtrace to top line
			for tmpCusor := cursor; tmpCusor < len(metaSheet.Rows); tmpCusor++ {
				metaSheet.Cell(tmpCusor, colName).Data = attrValue
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
		setKeyedType := !strings.HasPrefix(lastColName, prefix) || (len(matches) > 0 && repeated)
		if needChangeType {
			if matches := types.MatchMap(t); len(matches) == 3 {
				if !types.IsScalarType(matches[1]) && len(types.MatchEnum(matches[1])) == 0 {
					return errors.Errorf("%s is not scalar type in node %s attr %s type %s", matches[1], nav.LocalName(), attrName, t)
				}
				if matches[2] != nav.LocalName() {
					return errors.Errorf("%s in attr %s type %s must be the same as node name %s", matches[2], attrName, t, nav.LocalName())
				}
				metaSheet.SetColType(colName, t)
			} else if matches := types.MatchKeyedList(t); len(matches) == 3 {
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
			} else if matches := types.MatchList(t); len(matches) == 3 {
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
				if repeated {
					metaSheet.SetColType(colName, fmt.Sprintf("[%s]<%s>", strcase.ToCamel(nav.LocalName()), t))
				} else {
					metaSheet.SetColType(colName, fmt.Sprintf("{%s}%s", strcase.ToCamel(nav.LocalName()), t))
				}
			} else {
				metaSheet.SetColType(colName, t)
			}
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
			// `TABLEAU` can only be placed in the first child node
			if xmlquery.FindOne(navCopy.Current(), "/TABLEAU") != nil {
				return errors.Errorf("`TABLEAU` found in node %s (index:%d) which is not the first child", tagName, count+1)
			}
			// duplicate means a list, should expand vertically
			row := metaSheet.NewRow()
			if err := x.parseNode(&navCopy, metaSheet, row.Index); err != nil {
				return errors.Wrapf(err, "parseNode for node %s (index:%d) failed", tagName, count+1)
			}
			nodeMap[tagName]++
		} else {
			if err := x.parseNode(&navCopy, metaSheet, cursor); err != nil {
				return errors.Wrapf(err, "parseNode for the first node %s failed", tagName)
			}
			nodeMap[tagName] = 1
		}
	}

	return nil
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
