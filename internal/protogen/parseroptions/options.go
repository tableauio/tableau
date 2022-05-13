package parseroptions

// Options follow the design of Functional Options(https://github.com/tmrts/go-patterns/blob/master/idiom/functional-options.md)
type Options struct {
	Nested bool
	// virtual type cell for supporting composite first field type of list element.
	//
	// NOTE: need to used with prefix.
	//
	// e.g.:
	// - fist field type is incell-struct: []{int32 Id, string Name}
	// - fist field type is struct: []Item
	// - fist field type is also list: [][]int
	// - fist field type is map: []map<int, Item>
	vTypeCells map[int]string // cursor -> virtual type cell
}

func (opts *Options) GetVTypeCell(cursor int) string {
	if opts.vTypeCells == nil {
		return ""
	}
	return opts.vTypeCells[cursor]
}

// Option is the functional option type.
type Option func(*Options)

func Nested(nested bool) Option {
	return func(opts *Options) {
		opts.Nested = nested
	}
}

func VTypeCell(cursor int, typeCell string) Option {
	return func(opts *Options) {
		if opts.vTypeCells == nil {
			opts.vTypeCells = make(map[int]string)
		}
		opts.vTypeCells[cursor] = typeCell
	}
}

func newDefaultOptions() *Options {
	return &Options{
		Nested: false,
	}
}

func ParseOptions(options ...Option) *Options {
	// Default Options
	opts := newDefaultOptions()
	for _, setter := range options {
		setter(opts)
	}
	return opts
}
