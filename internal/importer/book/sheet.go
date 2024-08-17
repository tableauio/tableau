package book

import (
	"bytes"
	"strings"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
)

// MetasheetName is the name of metasheet which defines the metadata
// of each worksheet. Default is "@TABLEAU".
var MetasheetName = "@TABLEAU"

const SheetKey = "@sheet"

// BookNameInMetasheet is the special sign which represents workbook itself in metasheet.
// Default is "#".
const BookNameInMetasheet = "#"

// SetMetasheetName change the metasheet name to the specified name.
//
// NOTE: If will not change MetasheetName value if the specified name
// is empty.
func SetMetasheetName(name string) {
	if name != "" {
		MetasheetName = name
	}
}

type SheetParser interface {
	Parse(protomsg proto.Message, sheet *Sheet) error
}

type Sheet struct {
	Name string
	// Table represents data structure of 2D flat table formats.
	// E.g.: Excel, CSV.
	Table *Table
	// Document represents data structure of tree document formats.
	// E.g.: XML, YAML.
	Document *Node
	// Meta represents sheet's metadata, containing sheet’s layout,
	// parser options, loader options, and so on.
	Meta *tableaupb.Metasheet
}

// NewTableSheet creates a new Sheet with a table.
func NewTableSheet(name string, rows [][]string) *Sheet {
	return &Sheet{
		Name:  name,
		Table: NewTable(rows),
	}
}

// NewDocumentSheet creats a new Sheet with a document.
func NewDocumentSheet(name string, doc *Node) *Sheet {
	return &Sheet{
		Name:     name,
		Document: doc,
	}
}

// ParseMetasheet parses a sheet to Metabook by the specified parser.
func (s *Sheet) ParseMetasheet(parser SheetParser) (*tableaupb.Metabook, error) {
	metabook := &tableaupb.Metabook{}
	if s.Document != nil || (s.Table != nil && s.Table.MaxRow > 1) {
		if err := parser.Parse(metabook, s); err != nil {
			return nil, errors.WithMessagef(err, "failed to parse metasheet")
		}
	}
	return metabook, nil
}

// String returns the string representation of the Sheet, mainly
// for debugging.
//
//   - Table: CSV form
//   - Document: hierachy form
func (s *Sheet) String() string {
	if s.Document != nil {
		var buffer bytes.Buffer
		dumpNode(s.Document, DocumentNode, &buffer, 0)
		return buffer.String()
	} else if s.Table != nil {
		return s.Table.String()
	} else {
		return "empty: no table or document"
	}
}

func (s *Sheet) GetName() string {
	if s.Document != nil {
		return s.Document.GetDataSheetName()
	} else {
		return s.Name
	}
}

func (s *Sheet) GetDebugName() string {
	if s.Meta.Alias != "" {
		return s.GetName() + " (alias: " + s.Meta.Alias + ")"
	}
	return s.GetName()

}

func (s *Sheet) GetProtoName() string {
	if s.Meta.Alias != "" {
		return s.Meta.Alias
	}
	return s.GetName()
}

func (s *Sheet) ToWorkseet() *tableaupb.Worksheet {
	return &tableaupb.Worksheet{
		Options: &tableaupb.WorksheetOptions{
			Name:          s.GetName(),
			Namerow:       s.Meta.Namerow,
			Typerow:       s.Meta.Typerow,
			Noterow:       s.Meta.Noterow,
			Datarow:       s.Meta.Datarow,
			Transpose:     s.Meta.Transpose,
			Tags:          "",
			Nameline:      s.Meta.Nameline,
			Typeline:      s.Meta.Typeline,
			Nested:        s.Meta.Nested,
			Sep:           s.Meta.Sep,
			Subsep:        s.Meta.Subsep,
			Merger:        s.Meta.Merger,
			AdjacentKey:   s.Meta.AdjacentKey,
			FieldPresence: s.Meta.FieldPresence,
			Template:      s.Meta.Template,
			Mode:          s.Meta.Mode,
			Scatter:       s.Meta.Scatter,
			Optional:      s.Meta.Optional,
			Patch:         s.Meta.Patch,
			// Loader options:
			OrderedMap: s.Meta.OrderedMap,
			Index:      parseIndexes(s.Meta.Index),
		},
		Fields: []*tableaupb.Field{},
		Name:   s.GetProtoName(),
	}
}

func parseIndexes(str string) []string {
	if strings.TrimSpace(str) == "" {
		return nil
	}

	var indexes []string
	var hasGroupLeft, hasGroupRight bool
	start := 0
	for i := 0; i <= len(str); i++ {
		if i == len(str) {
			indexes = appendIndex(indexes, str, start, i)
			break
		}

		switch str[i] {
		case '(':
			hasGroupLeft = true
		case ')':
			hasGroupRight = true
		case ',':
			if (!hasGroupLeft && !hasGroupRight) || (hasGroupLeft && hasGroupRight) {
				indexes = appendIndex(indexes, str, start, i)

				start = i + 1 // skip ',' to next rune
				hasGroupLeft, hasGroupRight = false, false
			}
		}
	}
	return indexes
}

func appendIndex(indexes []string, str string, start, end int) []string {
	index := strings.TrimSpace(str[start:end])
	if index != "" {
		indexes = append(indexes, index)
	}
	return indexes
}

func MetasheetOptions() *tableaupb.WorksheetOptions {
	return &tableaupb.WorksheetOptions{
		Name:    MetasheetName,
		Namerow: 1,
		Datarow: 2,
	}
}
