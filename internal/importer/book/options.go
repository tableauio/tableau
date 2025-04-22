package book

type TableOptions struct {
	RowOffset int
}

// Option is the functional option type.
type TableOption func(*TableOptions)

// RowOffset sets the offset of table's first row to the whole sheet.
// For non-multi sheet modes, the offset is always zero.
func RowOffset(offset int) TableOption {
	return func(opts *TableOptions) {
		opts.RowOffset = offset
	}
}

func parseTableOptions(setters ...TableOption) *TableOptions {
	opts := &TableOptions{}
	for _, setter := range setters {
		setter(opts)
	}
	return opts
}
