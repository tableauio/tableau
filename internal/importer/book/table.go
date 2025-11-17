package book

import (
	"bytes"
	"encoding/csv"
	"io"

	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/xerrors"
	"github.com/xuri/excelize/v2"
)

type Tabler interface {
	BeginRow() int
	EndRow() int
	BeginCol() int
	EndCol() int
	RowSize() int
	ColSize() int
	Cell(row, col int) (string, error)
	GetRow(row int) []string
	GetCol(col int) []string
	Position(row, col int) string
}

// Table represents a 2D array table.
type Table struct {
	Rows           [][]string // 2D array strings
	maxRow, maxCol int        // max row and col count
	opts           TableOptions
}

// NewTable creates a new Table.
func NewTable(rows [][]string, setters ...TableOption) *Table {
	maxRow := len(rows)
	maxCol := 0
	// NOTE: different rows may have different lengths,
	// and we need to find the max col.
	for _, row := range rows {
		n := len(row)
		if n > maxCol {
			maxCol = n
		}
	}
	return &Table{
		Rows:   rows,
		maxRow: maxRow,
		maxCol: maxCol,
		opts:   *parseTableOptions(setters...),
	}
}

// BeginRow returns the begin row of the given block.
func (t *Table) BeginRow() int {
	if t.opts.BeginRow >= 0 {
		return t.opts.BeginRow
	}
	return 0
}

// EndRow returns the end row (row after the last row) of the given block.
//
// For loop example:
//
//	for row := t.BeginRow(); row < t.EndRow(); row++ {
//		fmt.Println(row)
//	}
func (t *Table) EndRow() int {
	if t.opts.EndRow > 0 && t.opts.EndRow <= t.maxRow {
		return t.opts.EndRow
	}
	return t.maxRow
}

// BeginCol returns the begin column of the given block.
func (t *Table) BeginCol() int {
	if t.opts.BeginCol >= 0 {
		return t.opts.BeginCol
	}
	return 0
}

// EndCol returns the end column (column after the last column) of the given block.
//
// For loop example:
//
//	for col := t.BeginCol(); row < t.EndCol(); col++ {
//		fmt.Println(col)
//	}
func (t *Table) EndCol() int {
	if t.opts.EndCol > 0 && t.opts.EndCol <= t.maxCol {
		return t.opts.EndCol
	}
	return t.maxCol
}

// RowSize returns the row size of the given block.
func (t *Table) RowSize() int {
	return t.EndRow() - t.BeginRow()
}

// ColSize returns the column size of the given block.
func (t *Table) ColSize() int {
	return t.EndCol() - t.BeginCol()
}

// GetRow returns the row data by row index (started with 0). It will return
// nil if not found.
func (t *Table) GetRow(row int) []string {
	if row >= len(t.Rows) {
		return nil
	}
	return t.Rows[row]
}

// GetCol returns the column data by column index (started with 0). It will
// return nil if not found.
func (t *Table) GetCol(col int) []string {
	if col >= t.maxCol {
		return nil
	}
	var data []string
	for _, row := range t.Rows {
		if col >= len(row) {
			data = append(data, "")
		} else {
			data = append(data, row[col])
		}
	}
	return data
}

// IsRowEmpty checks whether the whole row is empty.
func (t *Table) IsRowEmpty(row int) bool {
	if row >= len(t.Rows) {
		return true
	}
	for _, cell := range t.Rows[row] {
		if cell != "" {
			return false
		}
	}
	return true
}

// FindBlockEndRow finds the end row (row after the last non-empty row) of
// the block. If the start row is empty, it will just return the start row.
// Otherwise, it will return past-the-last non-empty row.
//
// NOTE: A block is a series of contiguous non-empty rows. So different blocks
// are seperated by one or more empty rows.
func (t *Table) FindBlockEndRow(startRow int) int {
	for row := startRow; row < t.EndRow(); row++ {
		if t.IsRowEmpty(row) {
			return row
		}
	}
	return t.EndRow()
}

// Cell returns the cell value at (row, col).
func (t *Table) Cell(row, col int) (string, error) {
	if row < t.BeginRow() || row >= t.EndRow() {
		return "", xerrors.Errorf("cell row %d out of range", row)
	}
	if col < t.BeginCol() || col >= t.EndCol() {
		return "", xerrors.Errorf("cell col %d out of range", col)
	}
	// NOTE: different row may have different length.
	if col >= len(t.Rows[row]) {
		return "", nil
	}
	return t.Rows[row][col], nil
}

// String converts Table to CSV string. It is mainly used for debugging.
func (t *Table) String() string {
	var buffer bytes.Buffer
	w := csv.NewWriter(&buffer)
	err := w.WriteAll(t.Rows) // calls Flush internally
	if err != nil {
		panic(err)
	}
	return buffer.String()
}

// ExportCSV exports Table to writer in CSV format.
func (t *Table) ExportCSV(writer io.Writer) error {
	w := csv.NewWriter(writer)
	// FIXME(wenchy): will be something wrong if we add the empty cell?
	// TODO: deepcopy a new rows!
	for nrow, row := range t.Rows {
		for i := len(row); i < t.maxCol; i++ {
			// log.Debugf("add empty cell: %s", s.Name)
			row = append(row, "")
		}
		t.Rows[nrow] = row
	}
	// TODO: escape the cell value with `,` and `"`.
	return w.WriteAll(t.Rows) // calls Flush internally
}

// ExportExcel exports Table to excel sheet.
func (t *Table) ExportExcel(file *excelize.File, sheetName string) error {
	_, err := file.NewSheet(sheetName)
	if err != nil {
		return xerrors.Wrapf(err, "failed to create new sheet %s", sheetName)
	}
	// TODO: clean up the sheet by using RemoveRow API.
	for nrow, row := range t.Rows {
		// file.SetRowHeight(s.Name, nrow, 20)
		for ncol, cell := range row {
			colname, err := excelize.ColumnNumberToName(ncol + 1)
			if err != nil {
				return xerrors.Wrapf(err, "failed to convert column number %d to name", ncol+1)
			}
			err = file.SetColWidth(sheetName, colname, colname, 20)
			if err != nil {
				return xerrors.Wrapf(err, "failed to set column width %s", colname)
			}
			axis, err := excelize.CoordinatesToCellName(ncol+1, nrow+1)
			if err != nil {
				return xerrors.Wrapf(err, "failed to convert coordinates (%d,%d) to cell name", ncol+1, nrow+1)
			}
			err = file.SetCellValue(sheetName, axis, cell)
			if err != nil {
				return xerrors.Wrapf(err, "failed to set cell value %s", axis)
			}
		}
	}
	return nil
}

func (t *Table) Position(row, col int) string {
	return excel.Position(row, col)
}

func (t *Table) Transpose() *TransposedTable {
	return &TransposedTable{table: t}
}
