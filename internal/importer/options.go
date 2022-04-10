package importer

import (
	"github.com/tableauio/tableau/internal/importer/book"
)

type Options struct {
	Sheets []string         // sheet names to import
	Parser book.SheetParser // parser to parse the worksheet
	TopN   uint             // read top N rows, 0 means read all rows
}

// Option is the functional option type.
type Option func(*Options)

func Sheets(sheets []string) Option {
	return func(opts *Options) {
		opts.Sheets = sheets
	}
}

func Parser(parser book.SheetParser) Option {
	return func(opts *Options) {
		opts.Parser = parser
	}
}

func TopN(n uint) Option {
	return func(opts *Options) {
		opts.TopN = n
	}
}

func newDefaultOptions() *Options {
	return &Options{}
}

func parseOptions(setters ...Option) *Options {
	// Default Options
	opts := newDefaultOptions()
	for _, setter := range setters {
		setter(opts)
	}
	return opts
}
