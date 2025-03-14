package parseroptions

// Options follow the design of Functional Options(https://github.com/tmrts/go-patterns/blob/master/idiom/functional-options.md)
type Options struct {
	Nested bool
	// Whether the field property `union_fields` is valid
	UnionFieldsValid bool
	// virtual type cell for supporting composite first field type of list element.
	//
	// NOTE: need to used with prefix.
	//
	// e.g.:
	// - first field type is incell-struct: []{int32 Id, string Name}
	// - first field type is struct: []Item
	// - first field type is also list: [][]int
	// - first field type is map: []map<int, Item>
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

func UnionFieldsValid() Option {
	return func(opts *Options) {
		opts.UnionFieldsValid = true
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
