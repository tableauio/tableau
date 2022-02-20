package importer

import (
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
)

type SheetParser interface {
	Parse(protomsg proto.Message, sheet *Sheet, wsOpts *tableaupb.WorksheetOptions) error
}

type Options struct {
	Format format.Format         // file format: Excel, CSV, XML. Default: Excel.
	Sheets []string              // sheet names to import
	Parser SheetParser           // parser to parse the worksheet
	Header *options.HeaderOption // header settings.
}

// Option is the functional option type.
type Option func(*Options)

func Format(fmt format.Format) Option {
	return func(opts *Options) {
		opts.Format = fmt
	}
}

func Sheets(sheets []string) Option {
	return func(opts *Options) {
		opts.Sheets = sheets
	}
}

func Parser(sp SheetParser) Option {
	return func(opts *Options) {
		opts.Parser = sp
	}
}

func Header(header *options.HeaderOption) Option {
	return func(opts *Options) {
		opts.Header = header
	}
}

func newDefaultOptions() *Options {
	return &Options{
		Format: format.Excel,
	}
}

func parseOptions(setters ...Option) *Options {
	// Default Options
	opts := newDefaultOptions()
	for _, setter := range setters {
		setter(opts)
	}
	return opts
}
