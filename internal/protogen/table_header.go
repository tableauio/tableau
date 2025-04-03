package protogen

import (
	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/protogen/parseroptions"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
)

type tableHeader struct {
	*parseroptions.Header
	transpose bool

	nameRowData []string
	typeRowData []string
	noteRowData []string

	// runtime data
	validNames map[string]int // none-empty valid names: name -> cursor
}

func newTableHeader(sheetOpts *tableaupb.WorksheetOptions, bookOpts *tableaupb.WorkbookOptions, globalOpts *options.HeaderOption) *tableHeader {
	return &tableHeader{
		Header:    parseroptions.MergeHeader(sheetOpts, bookOpts, globalOpts),
		transpose: sheetOpts.Transpose,
	}
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
	return getCell(t.noteRowData, cursor, 1) // default note line is 1
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
		position1 := excel.Postion(t.NameRow-1, foundCursor)
		position2 := excel.Postion(t.NameRow-1, cursor)
		if t.transpose {
			position1 = excel.Postion(foundCursor, t.NameRow-1)
			position2 = excel.Postion(cursor, t.NameRow-1)
		}
		return xerrors.E0003(name, position1, position2)
	}
	return nil
}

func getCell(row []string, cursor int, line int) string {
	// empty cell may be not in list
	if cursor >= len(row) {
		return ""
	}
	return book.ExtractFromCell(row[cursor], line)
}
