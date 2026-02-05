package parseroptions

import (
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
)

type Header struct {
	NameRow  int
	TypeRow  int
	NoteRow  int
	DataRow  int
	NameLine int
	TypeLine int
	NoteLine int
	Sep      string
	Subsep   string
}

// MergeHeader merges sheet-level, book-level, and global-level header options into the final header options.
func MergeHeader(sheetOpts *tableaupb.WorksheetOptions, bookOpts *tableaupb.WorkbookOptions, globalOpts *options.HeaderOption) *Header {
	hdr := &Header{}
	// name row
	if sheetOpts.GetNamerow() != 0 {
		hdr.NameRow = int(sheetOpts.GetNamerow())
	} else if bookOpts.GetNamerow() != 0 {
		hdr.NameRow = int(bookOpts.GetNamerow())
	} else if globalOpts != nil && globalOpts.NameRow != 0 {
		hdr.NameRow = int(globalOpts.NameRow)
	} else {
		hdr.NameRow = options.DefaultNameRow
	}
	// type row
	if sheetOpts.GetTyperow() != 0 {
		hdr.TypeRow = int(sheetOpts.GetTyperow())
	} else if bookOpts.GetTyperow() != 0 {
		hdr.TypeRow = int(bookOpts.GetTyperow())
	} else if globalOpts != nil && globalOpts.TypeRow != 0 {
		hdr.TypeRow = int(globalOpts.TypeRow)
	} else {
		hdr.TypeRow = options.DefaultTypeRow
	}
	// note row
	if sheetOpts.GetNoterow() != 0 {
		hdr.NoteRow = int(sheetOpts.GetNoterow())
	} else if bookOpts.GetNoterow() != 0 {
		hdr.NoteRow = int(bookOpts.GetNoterow())
	} else if globalOpts != nil && globalOpts.NoteRow != 0 {
		hdr.NoteRow = int(globalOpts.NoteRow)
	} else {
		hdr.NoteRow = options.DefaultNoteRow
	}
	// data row
	if sheetOpts.GetDatarow() != 0 {
		hdr.DataRow = int(sheetOpts.GetDatarow())
	} else if bookOpts.GetDatarow() != 0 {
		hdr.DataRow = int(bookOpts.GetDatarow())
	} else if globalOpts != nil && globalOpts.DataRow != 0 {
		hdr.DataRow = int(globalOpts.DataRow)
	} else {
		hdr.DataRow = options.DefaultDataRow
	}
	// name line
	if sheetOpts.GetNameline() != 0 {
		hdr.NameLine = int(sheetOpts.GetNameline())
	} else if bookOpts.GetNameline() != 0 {
		hdr.NameLine = int(bookOpts.GetNameline())
	} else if globalOpts != nil && globalOpts.NameLine != 0 {
		hdr.NameLine = int(globalOpts.NameLine)
	} else {
		hdr.NameLine = 0
	}
	// type line
	if sheetOpts.GetTypeline() != 0 {
		hdr.TypeLine = int(sheetOpts.GetTypeline())
	} else if bookOpts.GetTypeline() != 0 {
		hdr.TypeLine = int(bookOpts.GetTypeline())
	} else if globalOpts != nil && globalOpts.TypeLine != 0 {
		hdr.TypeLine = int(globalOpts.TypeLine)
	} else {
		hdr.TypeLine = 0
	}
	// note line
	if sheetOpts.GetNoteline() != 0 {
		hdr.NoteLine = int(sheetOpts.GetNoteline())
	} else if bookOpts.GetNoteline() != 0 {
		hdr.NoteLine = int(bookOpts.GetNoteline())
	} else if globalOpts != nil && globalOpts.NoteLine != 0 {
		hdr.NoteLine = int(globalOpts.NoteLine)
	} else {
		hdr.NoteLine = 0
	}
	// sep
	if sheetOpts.GetSep() != "" {
		hdr.Sep = sheetOpts.GetSep()
	} else if bookOpts.GetSep() != "" {
		hdr.Sep = bookOpts.GetSep()
	} else if globalOpts != nil && globalOpts.Sep != "" {
		hdr.Sep = globalOpts.Sep
	} else {
		hdr.Sep = options.DefaultSep
	}
	// subsep
	if sheetOpts.GetSubsep() != "" {
		hdr.Subsep = sheetOpts.GetSubsep()
	} else if bookOpts.GetSubsep() != "" {
		hdr.Subsep = bookOpts.GetSubsep()
	} else if globalOpts != nil && globalOpts.Subsep != "" {
		hdr.Subsep = globalOpts.Subsep
	} else {
		hdr.Subsep = options.DefaultSubsep
	}
	return hdr
}

func (header *Header) parseTableCols(table book.Tabler) (map[int]*book.Column, book.ColumnLookupTable, error) {
	nameRow := table.BeginRow() + header.NameRow - 1
	typeRow := table.BeginRow() + header.TypeRow - 1
	columns := make(map[int]*book.Column, table.ColSize())
	lookupTable := make(book.ColumnLookupTable, table.ColSize())
	for col := table.BeginCol(); col < table.EndCol(); col++ {
		// parse names
		nameCell, err := table.Cell(nameRow, col)
		if err != nil {
			return nil, nil, xerrors.WrapKV(err, table.Position(nameRow, col))
		}
		name := book.ExtractFromCell(nameCell, header.NameLine)
		if name != "" {
			// parse lookup table
			if foundCol, ok := lookupTable[name]; ok {
				return nil, nil, xerrors.E0003(name, table.Position(nameRow, foundCol), table.Position(nameRow, col))
			}
			lookupTable[name] = col
		}
		// parse types
		typeCell, err := table.Cell(typeRow, col)
		if err != nil {
			return nil, nil, xerrors.WrapKV(err)
		}
		typ := book.ExtractFromCell(typeCell, header.TypeLine)
		columns[col] = &book.Column{
			Col:  col,
			Name: name,
			Type: typ,
		}
	}
	return columns, lookupTable, nil
}

func (header *Header) RangeTableDataRows(table book.Tabler, sheetName string, adjacentKey bool, fn func(*book.Row) error) error {
	columns, lookupTable, err := header.parseTableCols(table)
	if err != nil {
		return err
	}
	var prev *book.Row
	// [datarow, endRow]: data rows
	dataRow := table.BeginRow() + header.DataRow - 1
	for row := dataRow; row < table.EndRow(); row++ {
		curr := book.NewRow(row, prev, sheetName, lookupTable)
		for col := table.BeginCol(); col < table.EndCol(); col++ {
			data, err := table.Cell(row, col)
			if err != nil {
				return xerrors.WrapKV(err)
			}
			curr.AddCell(columns[col], data, adjacentKey)
		}
		ignored, err := curr.Ignored()
		if err != nil {
			return err
		}
		if ignored {
			curr.Free()
			continue
		}
		err = fn(curr)
		if err != nil {
			return err
		}
		if prev != nil {
			prev.Free()
		}
		prev = curr
	}
	if prev != nil {
		prev.Free()
	}
	return nil
}
