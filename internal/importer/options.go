package importer

import (
	"github.com/tableauio/tableau/internal/importer/book"
)

var defaultTopN uint = 20 // read top N rows, 0 means read all rows

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
	Mode   ImporterMode     // importer mode
	Merged bool             // this book is merged to the main book
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

func Mode(m ImporterMode) Option {
	return func(opts *Options) {
		opts.Mode = m
	}
}

func Merged(merged bool) Option {
	return func(opts *Options) {
		opts.Merged = merged
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
