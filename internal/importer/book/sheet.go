package book

import (
	"bytes"
	"context"
	"strings"

	"github.com/tableauio/tableau/internal/importer/metasheet"
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
func NewTableSheet(name string, rows [][]string) *Sheet {
	return &Sheet{
		Name:  name,
		Table: NewTable(rows),
	}
}

// NewDocumentSheet creates a new Sheet with a document.
func NewDocumentSheet(name string, doc *Node) *Sheet {
	return &Sheet{
		Name:     name,
		Document: doc,
	}
}

// SubSheet creates a new sub-sheet with the specified options.
func (s *Sheet) SubSheet(options ...TableOption) *Sheet {
	return &Sheet{
		Name:  s.Name,
		Table: s.Tabler().SubTable(options...),
		Meta:  s.Meta,
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
//  - Table: CSV form
//  - Document: hierachy form
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

// Tabler returns the table of the Sheet.
// If the sheet is not a table, returns nil.
// If the sheet is transposed, returns the transposed table, otherwise returns
// the original table.
func (s *Sheet) Tabler() Tabler {
	if s.Table == nil {
		return nil
	}
	if s.Meta.GetTranspose() {
		return s.Table.Transpose()
	} else {
		return s.Table
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
			Name: s.GetDataName(),

			Namerow: s.Meta.Namerow,
			Typerow: s.Meta.Typerow,
			Noterow: s.Meta.Noterow,
			Datarow: s.Meta.Datarow,

			Nameline: s.Meta.Nameline,
			Typeline: s.Meta.Typeline,
			Noteline: s.Meta.Noteline,

			Sep:                    s.Meta.Sep,
			Subsep:                 s.Meta.Subsep,
			Nested:                 s.Meta.Nested,
			Transpose:              s.Meta.Transpose,
			Labels:                 s.Meta.Labels,
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
			OrderedMap:   s.Meta.OrderedMap,
			Index:        parseIndexes(s.Meta.Index),
			OrderedIndex: parseIndexes(s.Meta.OrderedIndex),
			LangOptions:  s.Meta.LangOptions,
		},
	}
}

func parseIndexes(str string) []string {
	if strings.TrimSpace(str) == "" {
		return nil
	}

	var indexes []string
	splits := strings.Split(str, ",")
	curr := ""
	for _, s := range splits {
		if curr == "" {
			curr = s
		} else {
			curr += "," + s
		}
		if strings.Count(curr, "(") == strings.Count(curr, ")") &&
			strings.Count(curr, "<") == strings.Count(curr, ">") {
			indexes = append(indexes, strings.TrimSpace(curr))
			curr = ""
		}
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
		Name:    metasheet.FromContext(context).Name,
		Namerow: 1,
		Datarow: 2,
	}
}
