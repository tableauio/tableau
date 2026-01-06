package book

type TableOptions struct {
	BeginRow, EndRow int
	BeginCol, EndCol int
}

// TableOption is the functional option type for table.
type TableOption func(*TableOptions)

// Rows sets the table-occupied row range of the whole sheet: [begin, end).
func Rows(begin, end int) TableOption {
	return func(opts *TableOptions) {
		opts.BeginRow = begin
		opts.EndRow = end
	}
}

// Cols sets the table-occupied column range of the whole sheet: [begin, end).
func Cols(begin, end int) TableOption {
	return func(opts *TableOptions) {
		opts.BeginCol = begin
		opts.EndCol = end
	}
}

func parseTableOptions(options ...TableOption) *TableOptions {
	opts := &TableOptions{}
	for _, setter := range options {
		setter(opts)
	}
	return opts
}
