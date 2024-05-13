package protogen

import (
	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
)

type sheetHeader struct {
	meta    *tableaupb.Metasheet
	namerow []string
	typerow []string
	noterow []string

	// runtime data
	validNames map[string]int // none-empty valid names: name -> cursor
}

// getValidNameCell try best to get a none-empty cell, starting from
// the specified cursor. Current and subsequent empty cells are skipped
// to find the first none-empty name cell.
func (sh *sheetHeader) getValidNameCell(cursor *int) string {
	for *cursor < len(sh.namerow) {
		cell := getCell(sh.namerow, *cursor, sh.meta.Nameline)
		if cell == "" {
			*cursor++
			continue
		}
		return cell
	}
	return ""
}

func (sh *sheetHeader) getNameCell(cursor int) string {
	return getCell(sh.namerow, cursor, sh.meta.Nameline)
}

func (sh *sheetHeader) getTypeCell(cursor int) string {
	return getCell(sh.typerow, cursor, sh.meta.Typeline)
}
func (sh *sheetHeader) getNoteCell(cursor int) string {
	return getCell(sh.noterow, cursor, 1) // default note line is 1
}

// checkNameConflicts checks to keep sure each column name must be unique in name row.
func (sh *sheetHeader) checkNameConflicts(name string, cursor int) error {
	foundCursor, ok := sh.validNames[name]
	if !ok {
		sh.validNames[name] = cursor
		return nil
	}
	if foundCursor != cursor {
		position1 := excel.Postion(int(sh.meta.Namerow-1), foundCursor)
		position2 := excel.Postion(int(sh.meta.Namerow-1), cursor)
		if sh.meta.Transpose {
			position1 = excel.Postion(foundCursor, int(sh.meta.Namerow-1))
			position2 = excel.Postion(cursor, int(sh.meta.Typerow-1))
		}
		return xerrors.E1000(name, position1, position2)
	}
	return nil
}

func getCell(row []string, cursor int, line int32) string {
	// empty cell may be not in list
	if cursor >= len(row) {
		return ""
	}
	return book.ExtractFromCell(row[cursor], line)
}
