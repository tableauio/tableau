package importer

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/subchen/go-xmldom"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/types"
)

type XMLImporter struct {
	*book.Book
}

var (
	attrRegexp      *regexp.Regexp
	tagRegexp       *regexp.Regexp
	metasheetRegexp *regexp.Regexp
)

const (
	xmlProlog             = `<?xml version='1.0' encoding='UTF-8'?>`
	atTableauDisplacement = `ATABLEAU`
	atTypeDisplacement    = "ATYPE"
	ungreedyPropGroup     = `(\|\{[^\{\}]+\})?`                       // e.g.: |{default:"100"}
	metasheetItemBlock    = `<Item(\s+\S+\s*=\s*("\S+"|'\S+'))+\s*/>` // e.g.: <Item Sheet="XXXConf" Sep="|"/>
	sheetBlock            = `<%v(>(.*\n)*</%v>|\s*/>)`                // e.g.: <XXXConf>...</XXXConf>
)

func init() {
	attrRegexp = regexp.MustCompile(`\s*=\s*("|')` + types.TypeGroup + ungreedyPropGroup + `("|')`) // e.g.: = "int32|{range:"1,~"}"
	tagRegexp = regexp.MustCompile(`>` + types.TypeGroup + ungreedyPropGroup + `</`)                // e.g.: >int32|{range:"1,~"}</
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

func NewXMLImporter(filename string, sheets []string, parser book.SheetParser, mode ImporterMode, cloned bool) (*XMLImporter, error) {
	var book *book.Book
	var err error
	if mode == Protogen {
		book, err = readXMLBookWithOnlySchemaSheet(filename, parser)
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to read csv book: %s", filename)
		}
		if err := book.ParseMetaAndPurge(); err != nil {
			return nil, errors.WithMessage(err, "failed to parse metasheet")
		}
	} else {
		book, err = readXMLBook(filename, parser)
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to read csv book: %s", filename)
		}
	}

	// log.Debugf("book: %+v", book)
	return &XMLImporter{
		Book: book,
	}, nil
}

func readXMLBook(filename string, parser book.SheetParser) (*book.Book, error) {
	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(bookName, filename, parser)

	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	rawDocs, err := extractRawXMLDocuments(string(content))
	if err != nil {
		return nil, err
	}
	for _, rawDoc := range rawDocs {
		doc, err := xmldom.ParseXML(rawDoc)
		if err != nil {
			return nil, err
		}
		sheet, err := parseXMLSheet(doc, UnknownMode)
		if err != nil {
			return nil, errors.WithMessagef(err, "file: %s", filename)
		}
		newBook.AddSheet(sheet)
	}
	return newBook, nil
}

func readXMLBookWithOnlySchemaSheet(filename string, parser book.SheetParser) (*book.Book, error) {
	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(bookName, filename, parser)

	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	metasheet := splitXMLMetasheet(string(content))
	rawDocs, err := extractRawXMLDocuments(metasheet)
	if err != nil {
		return nil, err
	}
	for _, rawDoc := range rawDocs {
		doc, err := xmldom.ParseXML(rawDoc)
		if err != nil {
			return nil, err
		}
		sheet, err := parseXMLSheet(doc, Protogen)
		if err != nil {
			return nil, errors.WithMessagef(err, "file: %s", filename)
		}
		newBook.AddSheet(sheet)
	}
	return newBook, nil
}

func parseXMLSheet(doc *xmldom.Document, mode ImporterMode) (*book.Sheet, error) {
	name := doc.Root.Name
	if mode == Protogen {
		if name == atTableauDisplacement {
			return parseXMLMetaSheet(doc)
		}
		name = "@" + name
	}

	bnode := &book.Node{}
	bnode.Kind = book.MapNode
	bnode.Children = append(bnode.Children, &book.Node{
		Name:  book.SheetKey,
		Value: name,
	})

	rootNode := &book.Node{}
	if err := parseXMLNode(doc.Root, rootNode, mode); err != nil {
		return nil, errors.Wrapf(err, "parse xml node failed")
	}
	if structNode := rootNode.FindChild(book.KeywordStruct); structNode != nil {
		// used for protogen
		bnode.Children = append(bnode.Children, structNode.Children...)
	} else {
		// used for confgen
		bnode.Children = append(bnode.Children, rootNode.Children...)
	}

	sheet := book.NewDocumentSheet(
		name,
		&book.Node{
			Name:     name,
			Kind:     book.DocumentNode,
			Children: []*book.Node{bnode},
		},
	)
	return sheet, nil
}

func parseXMLMetaSheet(doc *xmldom.Document) (*book.Sheet, error) {
	bnode := &book.Node{}
	bnode.Kind = book.MapNode
	bnode.Children = append(bnode.Children, &book.Node{
		Name:  book.SheetKey,
		Value: book.MetasheetName,
	})
	for _, child := range doc.Root.Children {
		if child.Name != "Item" {
			continue
		}
		subNode := &book.Node{
			Kind: book.MapNode,
		}
		for _, attribute := range child.Attributes {
			if attribute.Name == "Sheet" {
				subNode.Name = attribute.Value
			} else {
				subNode.Children = append(subNode.Children, &book.Node{
					Name:  attribute.Name,
					Value: attribute.Value,
				})
			}
		}
		bnode.Children = append(bnode.Children, subNode)
	}
	sheet := book.NewDocumentSheet(
		book.MetasheetName,
		&book.Node{
			Name:     book.MetasheetName,
			Kind:     book.DocumentNode,
			Children: []*book.Node{bnode},
		},
	)
	return sheet, nil
}

func parseXMLNode(node *xmldom.Node, bnode *book.Node, mode ImporterMode) error {
	switch mode {
	case Protogen:
		bnode.Kind = book.MapNode
		bnode.Name = node.Name
		if typeAttr := node.GetAttribute(atTypeDisplacement); typeAttr != nil {
			// predefined struct
			if len(node.Attributes) != 1 || len(node.Children) != 0 || node.Text != "" {
				return errors.Errorf("predefined struct should not have children, text, or other attributes|name: %s", node.Name)
			}
			bnode.Kind = book.MapNode
			bnode.Children = append(bnode.Children, &book.Node{
				Name:  book.KeywordType,
				Value: typeAttr.Value,
			})
			if desc := types.MatchList(typeAttr.Value); desc != nil {
				bnode.Children = append(bnode.Children, &book.Node{
					Name:  book.KeywordVariable,
					Value: fmt.Sprintf("%sList", bnode.Name),
				})
			}
			return nil
		}
		// NOTE: curBNode may be pointed to one subnode when needed
		curBNode := bnode
		for i, attr := range node.Attributes {
			var err error
			curBNode, err = parseXMLAttribute(curBNode, attr.Name, attr.Value, i == 0)
			if err != nil {
				return errors.WithMessagef(err, "parse xml attribute failed")
			}
		}
		// generate struct even if encounter empty node
		if len(node.Attributes) == 0 {
			curBNode = &book.Node{
				Kind: book.MapNode,
				Name: book.KeywordStruct,
			}
			bnode.Children = append(bnode.Children, &book.Node{
				Name:  book.KeywordType,
				Value: fmt.Sprintf("{%s}", node.Name),
			}, curBNode)
		}
		for _, child := range node.Children {
			if child.Text != "" {
				// child with text is regarded as an attribute
				if len(child.Attributes) != 0 || len(child.Children) != 0 {
					return errors.Errorf("node contains text so attributes and children must be empty|name: %s", child.Name)
				}
				_, err := parseXMLAttribute(curBNode, child.Name, child.Text, false)
				if err != nil {
					return errors.WithMessagef(err, "parse xml text-only child failed")
				}
				continue
			}
			subNode := &book.Node{}
			if err := parseXMLNode(child, subNode, mode); err != nil {
				return errors.Wrapf(err, "parse xml node failed")
			}
			curBNode.Children = append(curBNode.Children, subNode)
		}
	default:
		if node.Text != "" {
			if len(node.Attributes) != 0 || len(node.Children) != 0 {
				return errors.Errorf("node contains text so attributes and children must be empty|name: %s", node.Name)
			}
			bnode.Name = node.Name
			bnode.Value = node.Text
			break
		}
		bnode.Kind = book.MapNode
		bnode.Name = node.Name
		for _, attr := range node.Attributes {
			subNode := &book.Node{
				Name:  attr.Name,
				Value: attr.Value,
			}
			bnode.Children = append(bnode.Children, subNode)
		}
		for _, child := range node.Children {
			subNode := &book.Node{}
			if err := parseXMLNode(child, subNode, mode); err != nil {
				return errors.Wrapf(err, "parse xml node failed")
			}
			if subNode.Kind == book.ScalarNode {
				bnode.Children = append(bnode.Children, subNode)
			} else if existingBnode := bnode.FindChild(subNode.Name); existingBnode == nil {
				bnode.Children = append(bnode.Children, &book.Node{
					Kind: book.ListNode,
					Name: subNode.Name,
					Children: []*book.Node{
						{
							Kind:     book.MapNode,
							Children: subNode.Children,
						},
					},
				})
			} else {
				subNode.Name = ""
				existingBnode.Children = append(existingBnode.Children, subNode)
			}
		}
	}
	return nil
}

func parseXMLAttribute(bnode *book.Node, attrName, attrValue string, isFirstAttr bool) (*book.Node, error) {
	curBNode := bnode
	if desc := types.MatchMap(attrValue); desc != nil {
		valueDesc := types.ParseTypeDescriptor(desc.ValueType)
		switch valueDesc.Kind {
		case types.ScalarKind, types.EnumKind:
			// incell map
			if isFirstAttr {
				curBNode = &book.Node{
					Kind: book.MapNode,
					Name: book.KeywordStruct,
					Children: []*book.Node{
						{
							Kind: book.MapNode,
							Name: attrName,
							Children: []*book.Node{
								{
									Name:  book.KeywordType,
									Value: attrValue,
								},
								{
									Name:  book.KeywordVariable,
									Value: fmt.Sprintf("%sMap", attrName),
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
					Value: fmt.Sprintf("{%s}", bnode.Name),
				}, curBNode)
				return curBNode, nil
			}
			bnode.Children = append(bnode.Children, &book.Node{
				Kind: book.MapNode,
				Name: attrName,
				Children: []*book.Node{
					{
						Name:  book.KeywordType,
						Value: attrValue,
					},
					{
						Name:  book.KeywordVariable,
						Value: fmt.Sprintf("%sMap", attrName),
					},
					{
						Name:  book.KeywordIncell,
						Value: "true",
					},
				},
			})
			return bnode, nil
		default:
			if !isFirstAttr {
				return nil, errors.Errorf("vertical map not supported on non-first attributes")
			}
			// vertical map
			curBNode = &book.Node{
				Kind: book.MapNode,
				Name: book.KeywordStruct,
				Children: []*book.Node{
					{
						Name:  book.KeywordKey,
						Value: attrName,
					},
					{
						Name:  book.KeywordKeyname,
						Value: attrName,
					},
				},
			}
			bnode.Children = append(bnode.Children, &book.Node{
				Name:  book.KeywordType,
				Value: attrValue,
			}, &book.Node{
				Name:  book.KeywordVariable,
				Value: fmt.Sprintf("%sMap", bnode.Name),
			}, curBNode)
			return curBNode, nil
		}
	} else if desc := types.MatchList(attrValue); desc != nil {
		if desc.ElemType != "" && desc.ColumnType != "" {
			// struct list
			curBNode = &book.Node{
				Kind: book.MapNode,
				Name: book.KeywordStruct,
				Children: []*book.Node{
					{
						Name:  attrName,
						Value: desc.ColumnType,
					},
				},
			}
			if isFirstAttr {
				bnode.Children = append(bnode.Children, &book.Node{
					Name:  book.KeywordType,
					Value: fmt.Sprintf("[%s]", bnode.Name),
				}, &book.Node{
					Name:  book.KeywordVariable,
					Value: fmt.Sprintf("%sList", bnode.Name),
				}, curBNode)
				return curBNode, nil
			} else {
				bnode.Children = append(bnode.Children, curBNode.Children[0])
				return bnode, nil
			}
		} else if desc.ElemType != "" {
			// scalar or enum list
			curBNode = &book.Node{
				Kind: book.MapNode,
				Name: book.KeywordStruct,
				Children: []*book.Node{
					{
						Kind: book.MapNode,
						Name: attrName,
						Children: []*book.Node{
							{
								Name:  book.KeywordType,
								Value: attrValue,
							},
							{
								Name:  book.KeywordVariable,
								Value: fmt.Sprintf("%sList", attrName),
							},
						},
					},
				},
			}
			if isFirstAttr {
				bnode.Children = append(bnode.Children, &book.Node{
					Name:  book.KeywordType,
					Value: fmt.Sprintf("[%s]", bnode.Name),
				}, &book.Node{
					Name:  book.KeywordVariable,
					Value: fmt.Sprintf("%sList", bnode.Name),
				}, curBNode)
				return curBNode, nil
			} else {
				bnode.Children = append(bnode.Children, curBNode.Children[0])
				return bnode, nil
			}
		} else {
			// incell list
			curBNode = &book.Node{
				Kind: book.MapNode,
				Name: book.KeywordStruct,
				Children: []*book.Node{
					{
						Kind: book.MapNode,
						Name: attrName,
						Children: []*book.Node{
							{
								Name:  book.KeywordType,
								Value: fmt.Sprintf("[%s]", desc.ColumnType),
							},
							{
								Name:  book.KeywordVariable,
								Value: fmt.Sprintf("%sList", attrName),
							},
							{
								Name:  book.KeywordIncell,
								Value: "true",
							},
						},
					},
				},
			}
			if isFirstAttr {
				bnode.Children = append(bnode.Children, &book.Node{
					Name:  book.KeywordType,
					Value: fmt.Sprintf("{%s}", bnode.Name),
				}, curBNode)
				return curBNode, nil
			} else {
				bnode.Children = append(bnode.Children, curBNode.Children[0])
				return bnode, nil
			}
		}
	} else if desc := types.MatchStruct(attrValue); desc != nil {
		curBNode = &book.Node{
			Kind: book.MapNode,
			Name: book.KeywordStruct,
			Children: []*book.Node{
				{
					Name:  attrName,
					Value: desc.ColumnType,
				},
			},
		}
		if isFirstAttr {
			bnode.Children = append(bnode.Children, &book.Node{
				Name:  book.KeywordType,
				Value: fmt.Sprintf("{%s}", bnode.Name),
			}, curBNode)
			return curBNode, nil
		} else {
			bnode.Children = append(bnode.Children, curBNode)
			return bnode, nil
		}
	} else if isFirstAttr {
		// generate struct when first encounter scalar attribute
		curBNode = &book.Node{
			Kind: book.MapNode,
			Name: book.KeywordStruct,
			Children: []*book.Node{
				{
					Name:  attrName,
					Value: attrValue,
				},
			},
		}
		bnode.Children = append(bnode.Children, &book.Node{
			Name:  book.KeywordType,
			Value: fmt.Sprintf("{%s}", bnode.Name),
		}, curBNode)
		return curBNode, nil
	} else {
		bnode.Children = append(bnode.Children, &book.Node{
			Name:  attrName,
			Value: attrValue,
		})
		return bnode, nil
	}
}

// splitXMLMetasheet splits metasheet from xml notes
func splitXMLMetasheet(content string) string {
	matches := metasheetRegexp.FindStringSubmatch(content)
	if len(matches) < 2 {
		return ""
	}
	scanner := bufio.NewScanner(strings.NewReader(matches[0]))
	emptyLines := ""
	for scanner.Scan() {
		emptyLines += "\n"
	}
	if err := scanner.Err(); err != nil {
		log.Panicf("scanner err:%v", err)
		return ""
	}
	metasheet := matches[1]
	metasheet = strings.ReplaceAll(metasheet, book.MetasheetName, atTableauDisplacement)
	metasheet = strings.ReplaceAll(metasheet, book.KeywordType, atTypeDisplacement)
	metasheet = escapeAttrs(metasheet)
	metasheet = xmlProlog + "\n" + metasheet
	return metasheet
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
		matches := attrRegexp.FindStringSubmatch(s)
		var typeBuf, propBuf bytes.Buffer
		xml.EscapeText(&typeBuf, []byte(matches[2]))
		xml.EscapeText(&propBuf, []byte(matches[3]))
		return fmt.Sprintf("=\"%s%s\"", typeBuf.String(), propBuf.String())
	})
	escapedDoc = tagRegexp.ReplaceAllStringFunc(escapedDoc, func(s string) string {
		matches := tagRegexp.FindStringSubmatch(s)
		var typeBuf, propBuf bytes.Buffer
		xml.EscapeText(&typeBuf, []byte(matches[1]))
		xml.EscapeText(&propBuf, []byte(matches[2]))
		return fmt.Sprintf(">%s%s</", typeBuf.String(), propBuf.String())
	})
	return escapedDoc
}

// extractRawYAMLDocuments extracts raw XML into separate documents.
func extractRawXMLDocuments(content string) ([]string, error) {
	var results []string
	reader := strings.NewReader(content)
	decoder := xml.NewDecoder(reader)
	var buffer bytes.Buffer
	var encoder = xml.NewEncoder(&buffer)
	depth := 0
	for {
		t, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading token: %v", err)
		}
		switch tok := t.(type) {
		case xml.StartElement:
			depth++
			if err := encoder.EncodeToken(tok); err != nil {
				return nil, fmt.Errorf("error encoding %T token: %v", tok, err)
			}
		case xml.EndElement:
			depth--
			if err := encoder.EncodeToken(tok); err != nil {
				return nil, fmt.Errorf("error encoding %T token: %v", tok, err)
			}
			if depth == 0 {
				if err := encoder.Flush(); err != nil {
					return nil, fmt.Errorf("error flushing encoder: %v", err)
				}
				results = append(results, buffer.String())
				buffer.Reset()
				encoder = xml.NewEncoder(&buffer)
			}
		case xml.CharData:
			if err := encoder.EncodeToken(tok); err != nil {
				return nil, fmt.Errorf("error encoding %T token: %v", tok, err)
			}
		}
	}
	return results, nil
}
