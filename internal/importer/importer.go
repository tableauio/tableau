package importer

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/camelcase"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/xuri/excelize/v2"
)

type Importer interface {
	// GetSheet returns a Sheet of the specified sheet name.
	GetSheets() ([]*Sheet, error)
	GetSheet(name string) (*Sheet, error)
}

func New(filename string, setters ...Option) Importer {
	opts := parseOptions(setters...)
	switch opts.Format {
	case format.Excel:
		return NewExcelImporter(filename, opts.Sheets, opts.Parser, false)
	case format.CSV:
		return NewCSVImporter(filename)
	case format.XML:
		return NewXMLImporter(filename, opts.Header)
	default:
		return nil
	}
}

type Sheet struct {
	Name   string
	MaxRow int
	MaxCol int

	Rows [][]string // 2D array of string.

	Meta *tableaupb.SheetMeta
}

// NewSheet creats a new Sheet.
func NewSheet(name string, rows [][]string) *Sheet {
	maxRow := len(rows)
	maxCol := 0
	// MOTE: different row may have different length.
	// We need to find the max col.
	for _, row := range rows {
		n := len(row)
		if n > maxCol {
			maxCol = n
		}
	}
	return &Sheet{
		Name:   name,
		MaxRow: maxRow,
		MaxCol: maxCol,
		Rows:   rows,
	}
}

// Cell returns the cell at (row, col).
func (s *Sheet) Cell(row, col int) (string, error) {
	if row < 0 || row >= s.MaxRow {
		return "", errors.Errorf("row %d out of range", row)
	}
	if col < 0 || col >= s.MaxCol {
		return "", errors.Errorf("col %d out of range", col)
	}
	// MOTE: different row may have different length.
	if col >= len(s.Rows[row]) {
		return "", nil
	}
	return s.Rows[row][col], nil
}

// String returns the string representation (CSV) of the sheet.
func (s *Sheet) String() string {
	var buffer bytes.Buffer
	w := csv.NewWriter(&buffer)
	err := w.WriteAll(s.Rows) // calls Flush internally
	if err != nil {
		atom.Log.Panicf("write csv failed: %v", err)
	}
	return buffer.String()
}

func (s *Sheet) ExportCSV(writer io.Writer) error {
	w := csv.NewWriter(writer)
	// FIXME(wenchy): will be something wrong if we add the empty cell?
	// TODO: deepcopy a new rows!
	for nrow, row := range s.Rows {
		for i := len(row); i < s.MaxCol; i++ {
			// atom.Log.Debugf("add empty cell: %s", s.Name)
			row = append(row, "")
		}
		s.Rows[nrow] = row
	}
	// TODO: escape the cell value with `,` and `"`.
	return w.WriteAll(s.Rows) // calls Flush internally
}

func (s *Sheet) ExportExcel(file *excelize.File) error {
	file.NewSheet(s.Name)
	// TODO: clean up the sheet by using RemoveRow API.

	for nrow, row := range s.Rows {
		// file.SetRowHeight(s.Name, nrow, 20)
		for ncol, cell := range row {
			colname, err := excelize.ColumnNumberToName(ncol + 1)
			if err != nil {
				return errors.Wrapf(err, "failed to convert column number %d to name", ncol+1)
			}
			file.SetColWidth(s.Name, colname, colname, 20)

			axis, err := excelize.CoordinatesToCellName(ncol+1, nrow+1)
			if err != nil {
				return errors.Wrapf(err, "failed to convert coordinates (%d,%d) to cell name", ncol+1, nrow+1)
			}
			err = file.SetCellValue(s.Name, axis, cell)
			if err != nil {
				return errors.Wrapf(err, "failed to set cell value %s", axis)
			}
		}
	}
	return nil
}

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
	// atom.Log.Debugf("No enough lines in cell: %s, want at least %d lines", cell, line)
	return ""
}

type RowCells struct {
	// This previous row cells is for auto-populating the currernt row's missing data.
	// As the user doesn't fill the duplicate map key for easy use and clear reading.
	//
	// ServerName			ServerConfName
	// map<string, Server>	[Conf]string
	//
	// gamesvr				HeadFrameConf
	// activitysvr			ActivityConf
	// *MISSING-KEY*		ChapterConf
	// *MISSING-KEY*		CollectionConf

	prev *RowCells

	Row          int                 // row number
	cells        map[string]*RowCell // name -> RowCell
	indexedCells map[int]*RowCell    // column index -> RowCell
}

func NewRowCells(row int, prev *RowCells) *RowCells {
	return &RowCells{
		prev: prev,

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

func (r *RowCells) CellDebugString(name string) string {
	rc := r.Cell(name, false)
	if rc == nil {
		return fmt.Sprintf("(%d,%d)%s:%s", r.Row+1, -1, name, "")
	}
	dataFlag := ""
	if rc.autoPopulated {
		dataFlag = "~"
	}
	return fmt.Sprintf("(%d,%d)%s:%s%s", r.Row+1, rc.Col+1, name, rc.Data, dataFlag)
}

func (r *RowCells) SetCell(name string, col int, data, typ string) {
	cell := &RowCell{
		Col:  col,
		Data: data,
		Type: typ,
		Name: name,
	}

	// TODO: Parser(first-pass), check if this sheet is nested.
	if data == "" {
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
					atom.Log.Errorf("failed to find prev cell for name: %s, row: %d", name, r.Row)
				}
			}
		}
	}

	// add new cell
	r.cells[name] = cell
	r.indexedCells[col] = cell
}

func (r *RowCells) GetCellCountWithPrefix(prefix string) int {
	// atom.Log.Debug("name prefix: ", prefix)
	size := 0
	for name := range r.cells {
		if strings.HasPrefix(name, prefix) {
			num := 0
			// atom.Log.Debug("name: ", name)
			colSuffix := name[len(prefix):]
			// atom.Log.Debug("name: suffix ", colSuffix)
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
