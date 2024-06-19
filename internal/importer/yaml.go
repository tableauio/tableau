package importer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/log"
	"gopkg.in/yaml.v3"
)

type YAMLImporter struct {
	*book.Book
}

func NewYAMLImporter(filename string, sheetNames []string, parser book.SheetParser, mode ImporterMode, cloned bool) (*YAMLImporter, error) {
	book, err := readYAMLBook(filename, parser)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to read csv book: %s", filename)
	}

	if mode == Protogen {
		if err := book.ParseMetaAndPurge(); err != nil {
			return nil, errors.WithMessage(err, "failed to parse metasheet")
		}
	}

	return &YAMLImporter{
		Book: book,
	}, nil
}

func readYAMLBook(filename string, parser book.SheetParser) (*book.Book, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(bookName, filename, parser)

	// parse all documents in a file
	decoder := yaml.NewDecoder(file)
	for {
		var doc yaml.Node
		// Decode one document at a time
		err = decoder.Decode(&doc)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			} else {
				return nil, err
			}
		}
		sheet, err := parseYAMLSheet(&doc)
		if err != nil {
			return nil, errors.WithMessagef(err, "%s", filename)
		}
		newBook.AddSheet(sheet)
	}

	return newBook, nil
}

func parseYAMLSheet(doc *yaml.Node) (*book.Sheet, error) {
	bdoc := &book.Node{}
	err := parseYAMLNode(doc, bdoc, &bdoc.Name)
	if err != nil {
		return nil, err
	}
	sheet := book.NewSheetWithDocument(
		bdoc.Name,
		bdoc,
	)
	return sheet, nil
}

func parseYAMLNode(node *yaml.Node, bnode *book.Node, sheetName *string) error {
	switch node.Kind {
	case yaml.DocumentNode:
		bnode.Kind = book.DocumentNode
		bnode.Content = node.Value
		for _, child := range node.Content {
			subNode := &book.Node{
				Content: child.Value,
			}
			if err := parseYAMLNode(child, subNode, &bnode.Name); err != nil {
				return err
			}
			bnode.Children = append(bnode.Children, subNode)
		}
		return nil
	case yaml.MappingNode:
		bnode.Kind = book.MapNode
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			subNode := &book.Node{
				Name:    key.Value,
				Content: value.Value,
			}
			if subNode.Name == book.SheetKey {
				if *sheetName != "" {
					return fmt.Errorf("duplicate sheet name specified: %s -> %s", *sheetName, subNode.Content)
				}
				*sheetName = subNode.Content
			}
			bnode.Children = append(bnode.Children, subNode)
			if value.Kind == yaml.ScalarNode {
				continue
			}
			if err := parseYAMLNode(value, subNode, sheetName); err != nil {
				return err
			}
		}
		return nil
	case yaml.SequenceNode:
		bnode.Kind = book.ListNode
		for _, elem := range node.Content {
			subNode := &book.Node{
				Name:    "",
				Content: elem.Value,
			}
			bnode.Children = append(bnode.Children, subNode)
			if elem.Kind == yaml.ScalarNode {
				continue
			}
			if err := parseYAMLNode(elem, subNode, sheetName); err != nil {
				return err
			}
		}
		return nil
	case yaml.ScalarNode:
		log.Warnf("logic should not reach scalar node(%d:%d), value: %v, maybe encounter an empty document", node.Line, node.Column, node.Value)
		return nil
	default:
		return errors.Errorf("unknown yaml node(%d:%d) kind: %v, value: %v", node.Line, node.Column, node.Kind, node.Value)
	}
}