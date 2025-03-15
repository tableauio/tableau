package parseroptions

import "github.com/tableauio/tableau/proto/tableaupb"

// Options follow the design of Functional Options(https://github.com/tmrts/go-patterns/blob/master/idiom/functional-options.md)
type Options struct {
	Nested bool
	Mode   tableaupb.Mode
	// virtual type cell for supporting composite first field type of list element.
	//
	// NOTE: need to used with prefix.
	//
	// e.g.:
	// 	- first field type is incell-struct: []{int32 Id, string Name}
	// 	- first field type is struct: []Item
	// 	- first field type is also list: [][]int
	// 	- first field type is map: []map<int, Item>
	vTypeCells map[int]string // cursor -> virtual type cell
}

func (opts *Options) GetVTypeCell(cursor int) string {
	if opts.vTypeCells == nil {
		return ""
	}
	return opts.vTypeCells[cursor]
}

// IsUnionMode returns true if it belongs to following modes:
//   - MODE_UNION_TYPE
//   - MODE_UNION_TYPE_MULTI
func (opts *Options) IsUnionMode() bool {
	return opts.Mode == tableaupb.Mode_MODE_UNION_TYPE || opts.Mode == tableaupb.Mode_MODE_UNION_TYPE_MULTI
}

// Option is the functional option type.
type Option func(*Options)

func Nested(nested bool) Option {
	return func(opts *Options) {
		opts.Nested = nested
	}
}

func Mode(mode tableaupb.Mode) Option {
	return func(opts *Options) {
		opts.Mode = mode
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
