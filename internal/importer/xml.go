package importer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/subchen/go-xmldom"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/metasheet"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/xerrors"
)

type XMLImporter struct {
	*book.Book
}

var (
	attrRegexp *regexp.Regexp
	tagRegexp  *regexp.Regexp
)

const (
	xmlProlog          = `<?xml version='1.0' encoding='UTF-8'?>`
	atTypeDisplacement = "ATYPE"
	ungreedyPropGroup  = `(\|\{[^\{\}]+\})?` // e.g.: |{default:"100"}
	metasheetItemBlock = `<Item\s+[^>]*\/>`  // e.g.: <Item Sheet="XXXConf" Sep="|"/>
)

func init() {
	attrRegexp = regexp.MustCompile(`\s*=\s*("|')` + types.TypeGroup + ungreedyPropGroup + `("|')`) // e.g.: = "int32|{range:"1,~"}"
	tagRegexp = regexp.MustCompile(`>` + types.TypeGroup + ungreedyPropGroup + `</`)                // e.g.: >int32|{range:"1,~"}</
}

func NewXMLImporter(ctx context.Context, filename string, sheets []string, parser book.SheetParser, mode ImporterMode, cloned bool) (*XMLImporter, error) {
	var book *book.Book
	var err error
	if mode == Protogen {
		book, err = readXMLBookWithOnlySchemaSheet(ctx, filename, parser)
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to read xml book: %s", filename)
		}
		if err := book.ParseMetaAndPurge(); err != nil {
			return nil, xerrors.Wrapf(err, "failed to parse metasheet")
		}
	} else {
		book, err = readXMLBook(ctx, filename, parser)
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to read xml book: %s", filename)
		}
	}

	// log.Debugf("book: %+v", book)
	return &XMLImporter{
		Book: book,
	}, nil
}

func readXMLBook(ctx context.Context, filename string, parser book.SheetParser) (*book.Book, error) {
	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(ctx, bookName, filename, parser)

	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	ms := splitXMLMetasheet(string(content), metasheet.FromContext(ctx).MetasheetName())
	rawDocs, err := extractRawXMLDocuments(ms)
	if err != nil {
		return nil, err
	}
	for _, rawDoc := range rawDocs {
		doc, err := xmldom.ParseXML(rawDoc)
		if err != nil {
			return nil, err
		}
		sheet, err := parseXMLSheet(doc, Protogen, metasheet.FromContext(ctx).MetasheetName())
		if err != nil {
			return nil, xerrors.Wrapf(err, "file: %s", filename)
		}
		newBook.AddSheet(sheet)
	}
	rawDocs, err = extractRawXMLDocuments(string(content))
	if err != nil {
		return nil, err
	}
	for _, rawDoc := range rawDocs {
		doc, err := xmldom.ParseXML(rawDoc)
		if err != nil {
			return nil, err
		}
		sheet, err := parseXMLSheet(doc, UnknownMode, metasheet.FromContext(ctx).MetasheetName())
		if err != nil {
			return nil, xerrors.Wrapf(err, "file: %s", filename)
		}
		newBook.AddSheet(sheet)
	}
	return newBook, nil
}

func readXMLBookWithOnlySchemaSheet(ctx context.Context, filename string, parser book.SheetParser) (*book.Book, error) {
	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(ctx, bookName, filename, parser)

	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	ms := splitXMLMetasheet(string(content), metasheet.FromContext(ctx).MetasheetName())
	rawDocs, err := extractRawXMLDocuments(ms)
	if err != nil {
		return nil, err
	}
	for _, rawDoc := range rawDocs {
		doc, err := xmldom.ParseXML(rawDoc)
		if err != nil {
			return nil, err
		}
		sheet, err := parseXMLSheet(doc, Protogen, metasheet.FromContext(ctx).MetasheetName())
		if err != nil {
			return nil, xerrors.Wrapf(err, "file: %s", filename)
		}
		newBook.AddSheet(sheet)
	}
	return newBook, nil
}

func xmlMetasheetName(metasheetName string) string {
	return strings.Replace(metasheetName, "@", "AT", 1)
}

func parseXMLSheet(doc *xmldom.Document, mode ImporterMode, metasheetName string) (*book.Sheet, error) {
	name := doc.Root.Name
	if mode == Protogen {
		if name == xmlMetasheetName(metasheetName) {
			return parseXMLMetaSheet(doc, metasheetName)
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
		return nil, xerrors.Wrapf(err, "parse xml node failed")
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

func parseXMLMetaSheet(doc *xmldom.Document, metasheetName string) (*book.Sheet, error) {
	bnode := &book.Node{}
	bnode.Kind = book.MapNode
	bnode.Children = append(bnode.Children, &book.Node{
		Name:  book.SheetKey,
		Value: metasheetName,
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
		metasheetName,
		&book.Node{
			Name:     metasheetName,
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
				return xerrors.Errorf("predefined struct should not have children, text, or other attributes|name: %s", node.Name)
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
				return xerrors.Wrapf(err, "parse xml attribute failed")
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
					return xerrors.Errorf("node contains text so attributes and children must be empty|name: %s", child.Name)
				}
				_, err := parseXMLAttribute(curBNode, child.Name, child.Text, false)
				if err != nil {
					return xerrors.Wrapf(err, "parse xml text-only child failed")
				}
				continue
			}
			subNode := &book.Node{}
			if err := parseXMLNode(child, subNode, mode); err != nil {
				return xerrors.Wrapf(err, "parse xml node failed")
			}
			curBNode.Children = append(curBNode.Children, subNode)
		}
	default:
		bnode.Name = node.Name
		bnode.Value = node.Text
		if len(node.Attributes) == 0 && len(node.Children) == 0 {
			break
		}
		bnode.Kind = book.MapNode
		for _, attr := range node.Attributes {
			// treat attributes as scalar subnodes
			// Examples:
			//
			// <RankConf MaxScore="100">
			// </RankConf>
			//
			// will be converted to:
			//
			// # document RankConf
			//   MaxScore: 100 # scalar
			subNode := &book.Node{
				Name:  attr.Name,
				Value: attr.Value,
			}
			bnode.Children = append(bnode.Children, subNode)
		}
		for _, child := range node.Children {
			// treat children as list subnodes
			// Examples:
			//
			// <RankConf>
			//   <MaxScore>100</MaxScore>
			// </RankConf>
			//
			// will be converted to:
			//
			// # document RankConf
			//   MaxScore: # list
			// 	   - 100 # scalar
			//
			// <RankConf>
			//   <MaxScore>100</MaxScore>
			//   <MaxScore>200</MaxScore>
			// </RankConf>
			//
			// will be converted to:
			//
			// # document RankConf
			//   MaxScore: # list
			// 	   - 100 # scalar
			// 	   - 200 # scalar
			subNode := &book.Node{}
			if err := parseXMLNode(child, subNode, mode); err != nil {
				return xerrors.Wrapf(err, "parse xml node failed")
			}
			existingBnode := bnode.FindChild(subNode.Name)
			if existingBnode == nil {
				existingBnode = &book.Node{
					Kind: book.ListNode,
					Name: subNode.Name,
				}
				bnode.Children = append(bnode.Children, existingBnode)
			}
			if existingBnode.Kind != book.ListNode {
				return xerrors.Errorf("children name confilcts with attributes|name: %s", node.Name)
			}
			subNode.Name = ""
			existingBnode.Children = append(existingBnode.Children, subNode)
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
				return nil, xerrors.Errorf("vertical map not supported on non-first attributes")
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
						Value: desc.ColumnType + desc.Prop.RawProp(),
					},
				},
			}
			if isFirstAttr {
				bnode.Children = append(bnode.Children, &book.Node{
					Name:  book.KeywordType,
					Value: fmt.Sprintf("[%s]", desc.ElemType) + desc.Prop.RawProp(),
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
								Value: attrValue,
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
		switch {
		case desc.ColumnType == "", // predefined incell struct, e.g.: {.Item}
			strings.Contains(desc.StructType, " "): // incell struct, e.g.: {int32 ID, string Name}Item
			bnode.Children = append(bnode.Children, &book.Node{
				Kind: book.MapNode,
				Name: attrName,
				Children: []*book.Node{
					{
						Name:  book.KeywordType,
						Value: attrValue,
					},
					{
						Name:  book.KeywordIncell,
						Value: "true",
					},
				},
			})
			return bnode, nil
		default:
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
					Value: fmt.Sprintf("{%s}", desc.StructType),
				}, curBNode)
				return curBNode, nil
			} else {
				bnode.Children = append(bnode.Children, curBNode)
				return bnode, nil
			}
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
func splitXMLMetasheet(content string, metasheetName string) string {
	// metasheet regexp, e.g.:
	// <!--
	// <@TABLEAU>
	// 		<Item Sheet="Server" />
	// </@TABLEAU>

	// <Server>
	// 		<Weight Num="map<uint32, Weight>"/>
	// </Server>
	// -->
	metasheetRegexp := regexp.MustCompile(fmt.Sprintf(`<!--([\s\S]*?<%v(?:>(?:\s+`+metasheetItemBlock+`\s+)*</%v>|\s*/>)[\s\S]*?)-->`, metasheetName, metasheetName))
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
	metasheet = strings.ReplaceAll(metasheet, "<@", "<AT")
	metasheet = strings.ReplaceAll(metasheet, "</@", "</AT")
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
		err := xml.EscapeText(&typeBuf, []byte(matches[2]))
		if err != nil {
			log.Panicf("xml.EscapeText err: %v", err)
		}
		err = xml.EscapeText(&propBuf, []byte(matches[3]))
		if err != nil {
			log.Panicf("xml.EscapeText err: %v", err)
		}
		return fmt.Sprintf("=\"%s%s\"", typeBuf.String(), propBuf.String())
	})
	escapedDoc = tagRegexp.ReplaceAllStringFunc(escapedDoc, func(s string) string {
		matches := tagRegexp.FindStringSubmatch(s)
		var typeBuf, propBuf bytes.Buffer
		err := xml.EscapeText(&typeBuf, []byte(matches[1]))
		if err != nil {
			log.Panicf("xml.EscapeText err: %v", err)
		}
		err = xml.EscapeText(&propBuf, []byte(matches[2]))
		if err != nil {
			log.Panicf("xml.EscapeText err: %v", err)
		}
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
