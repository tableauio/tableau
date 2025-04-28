package book

// Option is the functional option type.
type TableOption func(*Table)

// Rows sets the table-occupied row range of the whole sheet.
func Rows(begin, end int) TableOption {
	return func(table *Table) {
		if begin >= 0 {
			table.BeginRow = begin
		}
		if end <= table.maxRow {
			table.EndRow = end
		}
	}
}

// Cols sets the table-occupied column range of the whole sheet.
func Cols(begin, end int) TableOption {
	return func(table *Table) {
		if begin >= 0 {
			table.BeginCol = begin
		}
		if end <= table.maxCol {
			table.EndCol = end
		}
	}
}
