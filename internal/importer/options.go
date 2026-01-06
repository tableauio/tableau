package importer

import (
	"github.com/tableauio/tableau/internal/importer/book"
)

var defaultTopN uint = 10 // read top N rows, 0 means read all rows

type ImporterMode int

// Importer mode
const (
	UnknownMode ImporterMode = 0
	Protogen    ImporterMode = 1
	Confgen     ImporterMode = 2
)

type Options struct {
	Sheets          []string         // sheet name patterns (by filepath.Match) to import
	Parser          book.SheetParser // parser to parse the worksheet
	Mode            ImporterMode     // importer mode
	Cloned          bool             // this book cloned (same schema different data) from the main book
	PrimaryBookName string           // if cloned, this is primary book name
}

// Option is the functional option type.
type Option func(*Options)

// Sheets specifies sheet name patterns (by filepath.Match) to import.
func Sheets(sheets []string) Option {
	return func(opts *Options) {
		opts.Sheets = sheets
	}
}

// Parser specifies parser to parse the worksheet.
func Parser(parser book.SheetParser) Option {
	return func(opts *Options) {
		opts.Parser = parser
	}
}

// Mode specifies importer mode.
func Mode(m ImporterMode) Option {
	return func(opts *Options) {
		opts.Mode = m
	}
}

// Cloned specifies this book cloned (same schema different data) from the main book.
func Cloned(primaryBookName string) Option {
	return func(opts *Options) {
		opts.Cloned = true
		opts.PrimaryBookName = primaryBookName
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
