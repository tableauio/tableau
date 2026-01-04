package book

import (
	"io"

	"github.com/xuri/excelize/v2"
)

type TransposedTable struct {
	table *Table
}

func (t *TransposedTable) BeginRow() int {
	return t.table.BeginCol()
}

func (t *TransposedTable) EndRow() int {
	return t.table.EndCol()
}

func (t *TransposedTable) BeginCol() int {
	return t.table.BeginRow()
}

func (t *TransposedTable) EndCol() int {
	return t.table.EndRow()
}

func (t *TransposedTable) RowSize() int {
	return t.table.ColSize()
}

func (t *TransposedTable) ColSize() int {
	return t.table.RowSize()
}

func (t *TransposedTable) Cell(row, col int) (string, error) {
	return t.table.Cell(col, row)
}

func (t *TransposedTable) String() string {
	return t.table.String()
}

func (t *TransposedTable) Position(row, col int) string {
	return t.table.Position(col, row)
}

func (t *TransposedTable) GetRow(row int) []string {
	return t.table.getCol(row)
}

func (t *TransposedTable) Transpose() Tabler {
	return t.table
}

func (t *TransposedTable) FindBlockEndRow(startRow int) int {
	return t.table.findBlockEndCol(startRow)
}

func (t *TransposedTable) SubTable(setters ...TableOption) Tabler {
	opts := parseTableOptions(setters...)
	opts.BeginCol, opts.BeginRow = opts.BeginRow, opts.BeginCol
	opts.EndCol, opts.EndRow = opts.EndRow, opts.EndCol
	return &TransposedTable{
		table: &Table{
			Rows:   t.table.Rows,
			maxCol: t.table.maxCol,
			maxRow: t.table.maxRow,
			opts:   *opts,
		},
	}
}

func (t *TransposedTable) ExportCSV(writer io.Writer) error {
	return t.table.ExportCSV(writer)
}

func (t *TransposedTable) ExportExcel(file *excelize.File, sheetName string) error {
	return t.table.ExportExcel(file, sheetName)
}
