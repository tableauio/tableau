package book

import (
	"bytes"
	"encoding/csv"
	"io"

	"github.com/tableauio/tableau/xerrors"
	"github.com/xuri/excelize/v2"
)

// Table represents a 2D array table.
type Table struct {
	MaxRow int
	MaxCol int
	Rows   [][]string // 2D array strings
}

// NewTable creates a new Table.
func NewTable(rows [][]string) *Table {
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
		MaxRow: maxRow,
		MaxCol: maxCol,
		Rows:   rows,
	}
}

// GetRow returns the row data by row index (started with 0). It will return
// nil if not found.
func (t *Table) GetRow(row int) []string {
	if row >= len(t.Rows) {
		return nil
	}
	return t.Rows[row]
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

// FindBlockEndRow finds the end row of the block. If the start row is empty,
// it will just return the start row. Otherwise, it will return the last
// none-empty row.
//
// NOTE: A block is a series of contiguous none-empty rows.
func (t *Table) FindBlockEndRow(startRow int) int {
	for row := startRow; row <= len(t.Rows); row++ {
		if t.IsRowEmpty(row) {
			if row == startRow {
				return row
			}
			return row - 1
		}
	}
	return len(t.Rows)
}

// ExtractBlock extracts a block of rows.
//
// NOTE: A block is a series of contiguous none-empty rows.
func (t *Table) ExtractBlock(startRow int) (rows [][]string, endRow int) {
	endRow = t.FindBlockEndRow(startRow)
	rows = t.Rows[startRow : endRow+1]
	return
}

// Cell returns the cell at (row, col).
func (t *Table) Cell(row, col int) (string, error) {
	if row < 0 || row >= t.MaxRow {
		return "", xerrors.Errorf("cell row %d out of range", row)
	}
	if col < 0 || col >= t.MaxCol {
		return "", xerrors.Errorf("cell col %d out of range", col)
	}
	// MOTE: different row may have different length.
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
		for i := len(row); i < t.MaxCol; i++ {
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
	file.NewSheet(sheetName)
	// TODO: clean up the sheet by using RemoveRow API.
	for nrow, row := range t.Rows {
		// file.SetRowHeight(s.Name, nrow, 20)
		for ncol, cell := range row {
			colname, err := excelize.ColumnNumberToName(ncol + 1)
			if err != nil {
				return xerrors.Wrapf(err, "failed to convert column number %d to name", ncol+1)
			}
			file.SetColWidth(sheetName, colname, colname, 20)
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
