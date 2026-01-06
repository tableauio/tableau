package importer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/xerrors"
	"gopkg.in/yaml.v3"
)

type YAMLImporter struct {
	*book.Book
}

func NewYAMLImporter(ctx context.Context, filename string, setters ...Option) (*YAMLImporter, error) {
	opts := parseOptions(setters...)
	var book *book.Book
	var err error
	if opts.Mode == Protogen {
		book, err = readYAMLBookWithOnlySchemaSheet(ctx, filename, opts.Parser)
		if err != nil {
			return nil, err
		}
		if err := book.ParseMetaAndPurge(); err != nil {
			return nil, xerrors.Wrapf(err, "failed to parse metasheet")
		}
	} else {
		book, err = readYAMLBook(ctx, filename, opts.Sheets, opts.Parser)
		if err != nil {
			return nil, err
		}
	}

	// log.Debugf("book: %+v", book)
	return &YAMLImporter{
		Book: book,
	}, nil
}

func readYAMLBook(ctx context.Context, filename string, sheetNames []string, parser book.SheetParser) (*book.Book, error) {
	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(ctx, bookName, filename, parser)
	file, err := os.Open(filename)
	if err != nil {
		return nil, xerrors.E3002(err)
	}
	// parse all documents in a file
	decoder := yaml.NewDecoder(file)
	for i := 0; ; i++ {
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
		sheet, err := parseYAMLSheet(&doc, i)
		if err != nil {
			return nil, xerrors.Wrapf(err, "file: %s", filename)
		}
		if ok, err := checkSheetWanted(sheet.Name, sheetNames); err != nil {
			return nil, xerrors.Wrapf(err, "failed to check sheet wanted: %s, sheetNames: %v", sheet.Name, sheetNames)
		} else if ok {
			newBook.AddSheet(sheet)
		}
	}
	return newBook, nil
}

func readYAMLBookWithOnlySchemaSheet(ctx context.Context, filename string, parser book.SheetParser) (*book.Book, error) {
	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(ctx, bookName, filename, parser)

	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	rawDocs, err := extractRawYAMLDocuments(string(content))
	if err != nil {
		return nil, err
	}
	for i, rawDoc := range rawDocs {
		if !isSchemaSheet(rawDoc) {
			continue
		}
		var doc yaml.Node
		err := yaml.Unmarshal([]byte(rawDoc), &doc)
		if err != nil {
			return nil, err
		}
		sheet, err := parseYAMLSheet(&doc, i)
		if err != nil {
			return nil, xerrors.Wrapf(err, "file: %s", filename)
		}
		newBook.AddSheet(sheet)
	}
	return newBook, nil
}

func parseYAMLSheet(doc *yaml.Node, index int) (*book.Sheet, error) {
	bdoc := &book.Node{}
	err := parseYAMLNode(doc, bdoc)
	if err != nil {
		return nil, err
	}
	sheetName := bdoc.GetMetaSheet()
	if sheetName == "" {
		// no sheet name specified, then auto generate it
		sheetName = fmt.Sprintf("Sheet%d", index)
	}
	bdoc.Name = sheetName
	sheet := book.NewDocumentSheet(
		sheetName,
		bdoc,
	)
	return sheet, nil
}

func parseYAMLNode(node *yaml.Node, bnode *book.Node) error {
	switch node.Kind {
	case yaml.DocumentNode:
		bnode.Kind = book.DocumentNode
		bnode.Value = node.Value
		for _, child := range node.Content {
			subNode := &book.Node{
				Value: child.Value,
				NamePos: book.Position{
					Line:   child.Line,
					Column: child.Column,
				},
				ValuePos: book.Position{
					Line:   child.Line,
					Column: child.Column,
				},
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
				Name:  key.Value,
				Value: value.Value,
				NamePos: book.Position{
					Line:   key.Line,
					Column: key.Column,
				},
				ValuePos: book.Position{
					Line:   value.Line,
					Column: value.Column,
				},
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
				Name:  "",
				Value: elem.Value,
				NamePos: book.Position{
					Line:   elem.Line,
					Column: elem.Column,
				},
				ValuePos: book.Position{
					Line:   elem.Line,
					Column: elem.Column,
				},
			}
			bnode.Children = append(bnode.Children, subNode)
			if elem.Kind == yaml.ScalarNode {
				continue
			}
			if err := parseYAMLNode(elem, subNode); err != nil {
				return err
			}
		}
		return nil
	case yaml.ScalarNode:
		log.Warnf("logic should not reach scalar node(%d:%d), value: %v, maybe encounter an empty document", node.Line, node.Column, node.Value)
		return nil
	default:
		return xerrors.Newf("unknown yaml node(%d:%d) kind: %v, value: %v", node.Line, node.Column, node.Kind, node.Value)
	}
}

var yamlSheetNameRegexp *regexp.Regexp

func init() {
	yamlSheetNameRegexp = regexp.MustCompile(`"@sheet"\s*:\s*(.+)`) // e.g.: "@sheet": "@EnvConf"
}

const yamlDocumentSeparator = "---"

// extractRawYAMLDocuments extracts raw YAML into separate documents.
func extractRawYAMLDocuments(content string) ([]string, error) {
	rawDocuments := strings.Split(content, yamlDocumentSeparator)
	var documents []string
	for _, doc := range rawDocuments {
		trimmedDoc := strings.TrimSpace(doc)
		if trimmedDoc != "" {
			documents = append(documents, trimmedDoc)
		}
	}
	return documents, nil
}

func isSchemaSheet(rawDoc string) bool {
	scanner := bufio.NewScanner(strings.NewReader(rawDoc))
	for scanner.Scan() {
		line := scanner.Text()
		matches := yamlSheetNameRegexp.FindStringSubmatch(line)
		if len(matches) == 2 {
			sheetName := strings.Trim(matches[1], `"`)
			// log.Debugf("sheet: %s", sheetName)
			return strings.HasPrefix(sheetName, "@")
		}
	}
	return false
}
