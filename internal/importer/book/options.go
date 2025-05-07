package book

type TableOptions struct {
	BeginRow, EndRow int
	BeginCol, EndCol int
}

// Option is the functional option type.
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

func parseTableOptions(setters ...TableOption) *TableOptions {
	opts := &TableOptions{}
	for _, setter := range setters {
		setter(opts)
	}
	return opts
}
