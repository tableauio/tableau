package book

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

func (t *TransposedTable) Position(row, col int) string {
	return t.table.Position(col, row)
}

func (t *TransposedTable) GetRow(row int) []string {
	return t.table.getCol(row)
}
