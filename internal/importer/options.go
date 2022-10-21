package importer

import (
	"github.com/tableauio/tableau/internal/importer/book"
)

type Mode int

// Importer mode
const (
	UnknownMode Mode = 0
	Protogen    Mode = 1
	Confgen     Mode = 2
)

type Options struct {
	Sheets []string         // sheet names to import
	Parser book.SheetParser // parser to parse the worksheet
	TopN   uint             // read top N rows, 0 means read all rows
	Mode   Mode             // importer mode, e.g.: Protogen
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

func ImporterMode(m Mode) Option {
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
