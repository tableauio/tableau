package importer

import (
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer/book"
)

type Importer interface {
	// Filename returns the parsed filename of the original inputed filename.
	// 	- Excel: same as the inputed filename.
	// 	- CSV: recognizes pattern: "<BookName>#<SheetName>.csv", and returns Glob name "<BookName>#*.csv".
	// 	- XML: same as the inputed filename.
	Filename() string
	// Bookname returns the book name after parsing the original inputed filename.
	// 	- Excel: the base filename without file extension.
	// 	- CSV: recognizes pattern: "<BookName>#<SheetName>.csv", and returns "<BookName>".
	// 	- XML: the base filename without file extension.
	BookName() string
	// GetSheets returns all sheets in order of the book.
	GetSheets() []*book.Sheet
	// GetSheet returns a Sheet of the specified sheet name.
	GetSheet(name string) *book.Sheet
}

func New(filename string, setters ...Option) (Importer, error) {
	opts := parseOptions(setters...)
	fmt := format.Ext2Format(filepath.Ext(filename))
	switch fmt {
	case format.Excel:
		return NewExcelImporter(filename, opts.Sheets, opts.Parser, opts.TopN)
	case format.CSV:
		return NewCSVImporter(filename, opts.Sheets, opts.Parser)
	case format.XML:
		return NewXMLImporter(filename, opts.Sheets)
	default:
		return nil, errors.Errorf("unsupported format: %d", fmt)
	}
}
