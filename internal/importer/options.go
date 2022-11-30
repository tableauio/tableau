package importer

import (
	"github.com/tableauio/tableau/internal/importer/book"
)

type ImporterMode int

// Importer mode
const (
	UnknownMode ImporterMode = 0
	Protogen    ImporterMode = 1
	Confgen     ImporterMode = 2
)

type Options struct {
	Sheets []string         // sheet names to import
	Parser book.SheetParser // parser to parse the worksheet
	TopN   uint             // read top N rows, 0 means read all rows
	Mode   ImporterMode     // importer mode
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

func Mode(m ImporterMode) Option {
	return func(opts *Options) {
		opts.Mode = m
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
