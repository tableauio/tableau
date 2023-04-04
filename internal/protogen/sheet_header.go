package protogen

import (
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/proto/tableaupb"
)

type sheetHeader struct {
	meta    *tableaupb.Metasheet
	namerow []string
	typerow []string
	noterow []string
}

func getCell(row []string, cursor int, line int32) string {
	// empty cell may be not in list
	if cursor >= len(row) {
		return ""
	}
	return book.ExtractFromCell(row[cursor], line)
}

func (sh *sheetHeader) getNameCell(cursor int) string {
	return getCell(sh.namerow, cursor, sh.meta.Nameline)
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

func (sh *sheetHeader) getTypeCell(cursor int) string {
	return getCell(sh.typerow, cursor, sh.meta.Typeline)
}
func (sh *sheetHeader) getNoteCell(cursor int) string {
	return getCell(sh.noterow, cursor, 1) // default note line is 1
}
