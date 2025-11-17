package protogen

import (
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/protogen/parseroptions"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
)

type tableHeader struct {
	*parseroptions.Header
	table book.Tabler

	nameRowData []string
	typeRowData []string
	noteRowData []string

	// runtime data
	validNames map[string]int // none-empty valid names: name -> cursor
}

func newTableHeader(sheetOpts *tableaupb.WorksheetOptions, bookOpts *tableaupb.WorkbookOptions, globalOpts *options.HeaderOption, table *book.Table) *tableHeader {
	header := &tableHeader{
		Header: parseroptions.MergeHeader(sheetOpts, bookOpts, globalOpts),
		table:  table,
	}
	if sheetOpts.Transpose {
		header.table = table.Transpose()
	}
	header.nameRowData = header.table.GetRow(header.table.BeginRow() + header.NameRow - 1)
	header.typeRowData = header.table.GetRow(header.table.BeginRow() + header.TypeRow - 1)
	header.noteRowData = header.table.GetRow(header.table.BeginRow() + header.NoteRow - 1)
	return header
}

// getValidNameCell try best to get a none-empty cell, starting from
// the specified cursor. Current and subsequent empty cells are skipped
// to find the first none-empty name cell.
func (t *tableHeader) getValidNameCell(cursor *int) string {
	for *cursor < len(t.nameRowData) {
		cell := getCell(t.nameRowData, *cursor, t.NameLine)
		if cell == "" {
			*cursor++
			continue
		}
		return cell
	}
	return ""
}

func (t *tableHeader) getNameCell(cursor int) string {
	return getCell(t.nameRowData, cursor, t.NameLine)
}

func (t *tableHeader) getTypeCell(cursor int) string {
	return getCell(t.typeRowData, cursor, t.TypeLine)
}

func (t *tableHeader) getNoteCell(cursor int) string {
	return getCell(t.noteRowData, cursor, t.NoteLine)
}

// checkNameConflicts checks to keep sure each column name must be unique in name row.
func (t *tableHeader) checkNameConflicts(name string, cursor int) error {
	if t.validNames == nil {
		t.validNames = map[string]int{}
	}
	foundCursor, ok := t.validNames[name]
	if !ok {
		t.validNames[name] = cursor
		return nil
	}
	if foundCursor != cursor {
		position1 := t.Position(t.NameRow-1, foundCursor)
		position2 := t.Position(t.NameRow-1, cursor)
		return xerrors.E0003(name, position1, position2)
	}
	return nil
}

func (t *tableHeader) Position(row, col int) string {
	return t.table.Position(row, col)
}

func getCell(row []string, cursor int, line int) string {
	// empty cell may be not in list
	if cursor >= len(row) {
		return ""
	}
	return book.ExtractFromCell(row[cursor], line)
}
