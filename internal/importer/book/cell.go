package book

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/internal/strcase/camelcase"
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

func ExtractFromCell(cell string, line int) string {
	if line == 0 {
		// line 0 means the whole cell.
		return clearNewline(strings.TrimSpace(cell))
	}

	lines := strings.Split(cell, "\n")
	if len(lines) >= line {
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

	Row         int                 // row number
	cells       map[uint32]*RowCell // column index (started with 0) -> RowCell
	lookupTable ColumnLookupTable   // name -> column index
}

func NewRowCells(row int, prev *RowCells, sheetName string) *RowCells {
	return &RowCells{
		SheetName: sheetName,
		prev:      prev,

		Row:   row,
		cells: make(map[uint32]*RowCell),
	}
}

func (rc *RowCells) Free() {
	for _, cell := range rc.cells {
		freeRowCell(cell)
	}
}

var cellPool *sync.Pool

func init() {
	cellPool = &sync.Pool{
		New: func() any {
			return new(RowCell)
		},
	}
}

func newRowCell(col int, name, typ *string, data string) *RowCell {
	cell := cellPool.Get().(*RowCell)
	// set
	cell.Col = col
	cell.Name = name
	cell.Type = typ
	cell.Data = data
	cell.autoPopulated = false
	return cell
}

func freeRowCell(cell *RowCell) {
	cellPool.Put(cell)
}

type RowCell struct {
	Col           int     // cell column index (0-based)
	Name          *string // cell name
	Type          *string // cell type
	Data          string  // cell data
	autoPopulated bool    // auto-populated
}

func (r *RowCell) GetName() string {
	if r.Name == nil {
		return ""
	}
	return *r.Name
}

func (r *RowCell) GetType() string {
	if r.Type == nil {
		return ""
	}
	return *r.Type
}

func (r *RowCells) Cell(name string, optional bool) (*RowCell, error) {
	var cell *RowCell
	col, ok := r.lookupTable[name]
	if ok {
		cell = r.cells[col]
	} else if optional {
		// if optional, return an empty cell.
		cell = &RowCell{
			Col:  -1,
			Data: "",
		}
	}
	if cell == nil {
		return nil, xerrors.E2014(name)
	}
	return cell, nil
}

func (r *RowCells) findCellRangeWithNamePrefix(prefix string) (left, right *RowCell) {
	minCol, maxCol := -1, -1
	for _, cell := range r.cells {
		if strings.HasPrefix(cell.GetName(), prefix) {
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
	return r.cells[uint32(minCol)], r.cells[uint32(maxCol)]
}

func (r *RowCells) CellDebugKV(name string) []any {
	col := "?"
	data := ""
	rc, err := r.Cell(name, false)
	if err != nil {
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

	return []any{
		xerrors.KeySheetName, r.SheetName,
		xerrors.KeyDataCellPos, pos,
		xerrors.KeyDataCell, data,
		xerrors.KeyColumnName, name,
	}
}

// column name -> column index (started with 0)
type ColumnLookupTable = map[string]uint32

func (r *RowCells) SetColumnLookupTable(table ColumnLookupTable) {
	r.lookupTable = table
}

func (r *RowCells) NewCell(col int, name, typ *string, data string, needPopulateKey bool) {
	cell := newRowCell(col, name, typ, data)
	// TODO: Parser(first-pass), check if this sheet is nested.
	if needPopulateKey && cell.Data == "" {
		if (types.IsMap(cell.GetType()) || types.IsKeyedList(cell.GetType())) && r.prev != nil {
			// NOTE: populate the missing map key from the prev row's corresponding cell.
			// TODO(wenchy): this is a flawed hack, need to be taken into more consideration.
			// Check: reverse backward to find the previous same nested-level keyed cell and
			// compare them to make sure they are the same.
			prefix := ""
			splits := camelcase.Split(cell.GetName())
			if len(splits) >= 2 {
				prefix = strings.Join(splits[:len(splits)-2], "")
			}
			needPopulate := false
			if prefix == "" {
				needPopulate = true
			} else {
				for i := cell.Col - 1; i >= 0; i-- {
					// prevData := r.prev.cells[col].Data
					ui := uint32(i)
					backCell := r.cells[ui]
					if !strings.HasPrefix(backCell.GetName(), prefix) {
						break
					}
					if types.IsMap(backCell.GetType()) || types.IsKeyedList(backCell.GetType()) {
						if r.prev.cells[ui].Data == r.cells[ui].Data {
							needPopulate = true
							break
						}
					}
				}
			}

			if needPopulate {
				if prevCell, err := r.prev.Cell(cell.GetName(), false); err != nil {
					log.Errorf("failed to find prev cell for name: %s, row: %d", name, r.Row)
				} else {
					cell.Data = prevCell.Data
					cell.autoPopulated = true
				}
			}
		}
	}

	// add new cell
	index := uint32(col)
	r.cells[index] = cell
}

func (r *RowCells) GetCellCountWithPrefix(prefix string) int {
	// log.Debug("name prefix: ", prefix)
	size := 0
	for _, cell := range r.cells {
		name := cell.GetName()
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
