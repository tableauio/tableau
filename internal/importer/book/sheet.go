package book

import (
	"bytes"
	"context"
	"strings"

	"github.com/tableauio/tableau/internal/metasheet"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
)

const SheetKey = "@sheet"

// BookNameInMetasheet is the special sign which represents workbook itself in metasheet.
// Default is "#".
const BookNameInMetasheet = "#"

type SheetParser interface {
	Parse(protomsg proto.Message, sheet *Sheet) error
}

type Sheet struct {
	Name string
	// Table represents the data structure of 2D flat table formats.
	// E.g.: Excel, CSV.
	Table *Table
	// Document represents the data structure of tree document formats.
	// E.g.: XML, YAML.
	Document *Node
	// Meta represents sheet's metadata, containing sheet’s layout,
	// parser options, loader options, and so on.
	Meta *internalpb.Metasheet
}

// NewTableSheet creates a new Sheet with a table.
func NewTableSheet(name string, rows [][]string, setters ...TableOption) *Sheet {
	return &Sheet{
		Name:  name,
		Table: NewTable(rows, setters...),
	}
}

// NewDocumentSheet creates a new Sheet with a document.
func NewDocumentSheet(name string, doc *Node) *Sheet {
	return &Sheet{
		Name:     name,
		Document: doc,
	}
}

// ParseMetasheet parses a sheet to Metabook by the specified parser.
func (s *Sheet) ParseMetasheet(parser SheetParser) (*internalpb.Metabook, error) {
	metabook := &internalpb.Metabook{}
	if s.Document != nil || (s.Table != nil && s.Table.RowSize() > 1) {
		// For table, parse it only if the table have data part (the first row
		// is the name row).
		if err := parser.Parse(metabook, s); err != nil {
			return nil, xerrors.Wrapf(err, "failed to parse metasheet")
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

// GetDataName returns original data sheet name by removing leading symbol "@"
// from meta sheet name. For example: "@SheetName" -> "SheetName".
func (s *Sheet) GetDataName() string {
	if s.Document != nil {
		return s.Document.GetDataSheetName()
	} else {
		return s.Name
	}
}

// GetDebugName returns sheet name with alias if specified.
func (s *Sheet) GetDebugName() string {
	if s.Meta.Alias != "" {
		return s.Name + " (alias: " + s.Meta.Alias + ")"
	}
	return s.Name
}

// GetDebugName returns this sheet's corresponding protobuf message name.
func (s *Sheet) GetProtoName() string {
	if s.Meta.Alias != "" {
		return s.Meta.Alias
	}
	return s.GetDataName()
}

// ToWorkseet creates a new basic internalpb.Worksheet without fields populated,
// based on this sheet's info.
func (s *Sheet) ToWorkseet() *internalpb.Worksheet {
	return &internalpb.Worksheet{
		Name: s.GetProtoName(),
		Note: "", // NOTE: maybe will be used in the future
		Options: &tableaupb.WorksheetOptions{
			Name:                   s.GetDataName(),
			Namerow:                s.Meta.Namerow,
			Typerow:                s.Meta.Typerow,
			Noterow:                s.Meta.Noterow,
			Datarow:                s.Meta.Datarow,
			Transpose:              s.Meta.Transpose,
			Labels:                 s.Meta.Labels,
			Nameline:               s.Meta.Nameline,
			Typeline:               s.Meta.Typeline,
			Nested:                 s.Meta.Nested,
			Sep:                    s.Meta.Sep,
			Subsep:                 s.Meta.Subsep,
			Merger:                 s.Meta.Merger,
			AdjacentKey:            s.Meta.AdjacentKey,
			FieldPresence:          s.Meta.FieldPresence,
			Template:               s.Meta.Template,
			Mode:                   s.Meta.Mode,
			Scatter:                s.Meta.Scatter,
			Optional:               s.Meta.Optional,
			Patch:                  s.Meta.Patch,
			WithParentDir:          s.Meta.WithParentDir,
			ScatterWithoutBookName: s.Meta.ScatterWithoutBookName,
			// Loader options:
			OrderedMap:  s.Meta.OrderedMap,
			Index:       parseIndexes(s.Meta.Index),
			LangOptions: s.Meta.LangOptions,
		},
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

func MetabookOptions() *tableaupb.WorkbookOptions {
	return &tableaupb.WorkbookOptions{
		Namerow: 1,
		Datarow: 2,
	}
}

func MetasheetOptions(context context.Context) *tableaupb.WorksheetOptions {
	return &tableaupb.WorksheetOptions{
		Name:    metasheet.FromContext(context).MetasheetName(),
		Namerow: 1,
		Datarow: 2,
	}
}
