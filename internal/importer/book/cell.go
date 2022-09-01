package book

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"

	"github.com/tableauio/tableau/internal/camelcase"
	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/xerrors"
)

var newlineRegex *regexp.Regexp

func init() {
	newlineRegex = regexp.MustCompile(`\r?\n?`)
}

func clearNewline(s string) string {
	return newlineRegex.ReplaceAllString(s, "")
}

func ExtractFromCell(cell string, line int32) string {
	if line == 0 {
		// line 0 means the whole cell.
		return clearNewline(strings.TrimSpace(cell))
	}

	lines := strings.Split(cell, "\n")
	if int32(len(lines)) >= line {
		return strings.TrimSpace(lines[line-1])
	}
	// log.Debugf("No enough lines in cell: %s, want at least %d lines", cell, line)
	return ""
}

type RowCells struct {
	// The previous row cells is for auto-populating the currernt row's missing data.
	// So user need not fill the duplicate map key for easy use and clear reading.
	//
	// ServerName			ServerConfName
	// map<string, Server>	[Conf]string
	//
	// gamesvr				HeadFrameConf
	// activitysvr			ActivityConf
	// *MISSING-KEY*		ChapterConf
	// *MISSING-KEY*		CollectionConf

	SheetName string
	prev      *RowCells

	Row          int                 // row number
	cells        map[string]*RowCell // name -> RowCell
	indexedCells map[int]*RowCell    // column index -> RowCell
}

func NewRowCells(row int, prev *RowCells, sheetName string) *RowCells {
	return &RowCells{
		SheetName: sheetName,
		prev:      prev,

		Row:          row,
		cells:        make(map[string]*RowCell),
		indexedCells: make(map[int]*RowCell),
	}
}

type RowCell struct {
	Col           int    // cell column (0-based)
	Data          string // cell data
	Type          string // cell type
	Name          string // cell name
	autoPopulated bool   // auto-populated
}

func (r *RowCells) Cell(name string, optional bool) *RowCell {
	c := r.cells[name]
	if c == nil && optional {
		// if optional, return an empty cell.
		c = &RowCell{
			Col:  -1,
			Data: "",
		}
	}
	return c
}

func (r *RowCells) findCellRangeWithNamePrefix(prefix string) (left, right *RowCell) {
	minCol, maxCol := -1, -1
	for name, cell := range r.cells {
		if strings.HasPrefix(name, prefix) {
			if minCol == -1 || minCol > cell.Col {
				minCol = cell.Col
			}
			if maxCol == -1 || maxCol < cell.Col {
				maxCol = cell.Col
			}
		}
	}
	if minCol == -1 || maxCol == -1 {
		return nil, nil
	}
	return r.indexedCells[minCol], r.indexedCells[maxCol]
}

func (r *RowCells) CellDebugString(name string) string {
	col := "?"
	data := ""
	rc := r.Cell(name, false)
	if rc == nil {
		left, right := r.findCellRangeWithNamePrefix(name)
		if left != nil && right != nil {
			col = fmt.Sprintf("[%s...%s]", excel.LetterAxis(left.Col), excel.LetterAxis(right.Col))
			data = fmt.Sprintf("[%s...%s]", left.Data, right.Data)
		}
	} else {
		data = rc.Data
		if rc.autoPopulated {
			data += "~"
		}
		col = excel.LetterAxis(rc.Col)
	}
	pos := fmt.Sprintf("%s%d", col, r.Row+1)

	return xerrors.CombineKV(
		xerrors.SheetName, r.SheetName,
		xerrors.CellPos, pos,
		xerrors.CellData, data,
		xerrors.ColumnName, name,
	)
}

func (r *RowCells) SetCell(name string, col int, data, typ string, needPopulateKey bool) {
	cell := &RowCell{
		Col:  col,
		Data: data,
		Type: typ,
		Name: name,
	}

	// TODO: Parser(first-pass), check if this sheet is nested.
	if needPopulateKey && data == "" {
		if (types.MatchMap(typ) != nil || types.MatchKeyedList(typ) != nil) && r.prev != nil {
			// NOTE: populate the missing map key from the prev row's corresponding cell.
			// TODO(wenchy): this is a flawed hack, need to be taken into more consideration.
			// Check: reverse backward to find the previous same nested-level keyed cell and
			// compare them to make sure they are the same.
			prefix := ""
			splits := camelcase.Split(name)
			if len(splits) >= 2 {
				prefix = strings.Join(splits[:len(splits)-2], "")
			}
			needPopulate := false
			if prefix == "" {
				needPopulate = true
			} else {
				for i := col - 1; i >= 0; i-- {
					// prevData := r.prev.indexedCells[col].Data
					backCell := r.indexedCells[i]
					if !strings.HasPrefix(backCell.Name, prefix) {
						break
					}
					if types.MatchMap(backCell.Type) != nil || types.MatchKeyedList(backCell.Type) != nil {
						if r.prev.indexedCells[i].Data == r.indexedCells[i].Data {
							needPopulate = true
							break
						}
					}
				}
			}

			if needPopulate {
				if prevCell := r.prev.Cell(name, false); prevCell != nil {
					cell.Data = prevCell.Data
					cell.autoPopulated = true
				} else {
					log.Errorf("failed to find prev cell for name: %s, row: %d", name, r.Row)
				}
			}
		}
	}

	// add new cell
	r.cells[name] = cell
	r.indexedCells[col] = cell
}

func (r *RowCells) GetCellCountWithPrefix(prefix string) int {
	// log.Debug("name prefix: ", prefix)
	size := 0
	for name := range r.cells {
		if strings.HasPrefix(name, prefix) {
			num := 0
			// log.Debug("name: ", name)
			colSuffix := name[len(prefix):]
			// log.Debug("name: suffix ", colSuffix)
			for _, r := range colSuffix {
				if unicode.IsDigit(r) {
					num = num*10 + int(r-'0')
				} else {
					break
				}
			}
			size = int(math.Max(float64(size), float64(num)))
		}
	}
	return size
}
