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
	"github.com/tableauio/tableau/internal/x/xproto"
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

type Column struct {
	Col  int    // column index (0-based)
	Name string // column name
	Type string // column type
}

var cellPool *sync.Pool

func init() {
	cellPool = &sync.Pool{
		New: func() any {
			return new(Cell)
		},
	}
}

func newCell(col *Column, data string) *Cell {
	cell := cellPool.Get().(*Cell)
	// set
	cell.Column = col
	cell.Data = data
	cell.autoPopulated = false
	return cell
}

func freeCell(cell *Cell) {
	cellPool.Put(cell)
}

// Cell represents a cell in the row of sheet.
type Cell struct {
	*Column
	Data          string // cell data
	autoPopulated bool   // auto-populated
}

func (c *Cell) GetName() string {
	if c.Column == nil {
		return ""
	}
	return c.Name
}

func (c *Cell) GetType() string {
	if c.Column == nil {
		return ""
	}
	return c.Type
}

// Row represents a row in the sheet.
type Row struct {
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
	prev      *Row

	Row         int               // row number
	cells       map[int]*Cell     // column index (started with 0) -> Cell
	lookupTable ColumnLookupTable // name -> column index
}

// NewRow creates a new row.
func NewRow(row int, prev *Row, sheetName string, lookupTable ColumnLookupTable) *Row {
	return &Row{
		SheetName: sheetName,
		prev:      prev,

		Row:         row,
		cells:       make(map[int]*Cell),
		lookupTable: lookupTable,
	}
}

// Free frees the row.
func (r *Row) Free() {
	for _, cell := range r.cells {
		freeCell(cell)
	}
}

// Cell returns the cell with the given name.
func (r *Row) Cell(name string, optional bool) (*Cell, error) {
	var cell *Cell
	col, ok := r.lookupTable[name]
	if ok {
		cell = r.cells[col]
	} else if optional {
		// if optional, return an empty cell.
		cell = &Cell{
			Column: &Column{Col: -1},
			Data:   "",
		}
	}
	if cell == nil {
		return nil, xerrors.E2014(name)
	}
	return cell, nil
}

func (r *Row) findCellRangeWithNamePrefix(prefix string) (left, right *Cell) {
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
	return r.cells[minCol], r.cells[maxCol]
}

// CellDebugKV returns a list of key-value pairs for debugging.
func (r *Row) CellDebugKV(name string) []any {
	col := "?"
	data := ""
	cell, err := r.Cell(name, false)
	if err != nil {
		left, right := r.findCellRangeWithNamePrefix(name)
		if left != nil && right != nil {
			col = fmt.Sprintf("[%s...%s]", excel.LetterAxis(left.Col), excel.LetterAxis(right.Col))
			data = fmt.Sprintf("[%s...%s]", left.Data, right.Data)
		}
	} else {
		data = cell.Data
		if cell.autoPopulated {
			data += "~"
		}
		col = excel.LetterAxis(cell.Col)
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
type ColumnLookupTable = map[string]int

// Macro command definitions
const MacroIgnore = "#IGNORE" // bool: whether to ignore row

// Ignored checkes whether this row is ignored.
func (r *Row) Ignored() (bool, error) {
	if ignoreCol, ok := r.lookupTable[MacroIgnore]; ok {
		value := strings.TrimSpace(r.cells[ignoreCol].Data)
		if value == "" {
			return false, nil
		}
		ignored, err := xproto.ParseBool(value)
		if err != nil {
			return false, xerrors.WrapKV(err, r.CellDebugKV(MacroIgnore)...)
		}
		return ignored, nil
	}
	return false, nil
}

// AddCell adds a cell to the row.
func (r *Row) AddCell(col *Column, data string, needPopulateKey bool) {
	cell := newCell(col, data)
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
					backCell := r.cells[i]
					if !strings.HasPrefix(backCell.GetName(), prefix) {
						break
					}
					if types.IsMap(backCell.GetType()) || types.IsKeyedList(backCell.GetType()) {
						if r.prev.cells[i].Data == r.cells[i].Data {
							needPopulate = true
							break
						}
					}
				}
			}

			if needPopulate {
				if prevCell, err := r.prev.Cell(cell.GetName(), false); err != nil {
					log.Errorf("failed to find prev cell for name: %s, row: %d", col.Name, r.Row)
				} else {
					cell.Data = prevCell.Data
					cell.autoPopulated = true
				}
			}
		}
	}

	// add new cell
	r.cells[col.Col] = cell
}

// GetCellCountWithPrefix returns the cell count with the given prefix.
func (r *Row) GetCellCountWithPrefix(prefix string) int {
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
