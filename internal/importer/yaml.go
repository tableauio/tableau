package importer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/importer/book"
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
			return nil, err
		}
		newBook.AddSheet(sheet)
	}

	return newBook, nil
}

func parseYAMLSheet(doc *yaml.Node) (*book.Sheet, error) {
	//doc := &book.Document{}
	bnode := &book.Node{}
	err := parseYAMLNode(doc, bnode)
	if err != nil {
		return nil, err
	}
	sheet := book.NewSheetWithDocument(
		bnode.Name,
		bnode,
	)
	return sheet, nil
}

func parseYAMLNode(node *yaml.Node, bnode *book.Node) error {
	switch node.Kind {
	case yaml.DocumentNode:
		bnode.Kind = book.DocumentNode
		bnode.Name = "xxxdoc" // TODO
		bnode.Content = node.Value
		for _, child := range node.Content {
			subNode := &book.Node{
				Name:    "", // TODO
				Content: child.Value,
			}
			if err := parseYAMLNode(child, subNode); err != nil {
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
			bnode.Children = append(bnode.Children, subNode)
			if value.Kind == yaml.ScalarNode {
				continue
			}
			if err := parseYAMLNode(value, subNode); err != nil {
				return err
			}
		}
		return nil
	case yaml.SequenceNode:
		bnode.Kind = book.ListNode
		for _, elem := range node.Content {
			subNode := &book.Node{
				Name:    "", // TODO
				Content: elem.Value,
			}
			if elem.Kind == yaml.ScalarNode {
				continue
			}
			if err := parseYAMLNode(elem, subNode); err != nil {
				return err
			}
			bnode.Children = append(bnode.Children, subNode)
		}
		return nil
	default:
		return fmt.Errorf("unknown yaml node kind: %v", node.Kind)
	}
}
