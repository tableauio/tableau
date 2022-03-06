package importer

import (
	"path/filepath"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer/book"
)

// MetaSheetName defines the meta data of each worksheet.
const MetaSheetName = "@TABLEAU"

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
	GetSheets() ([]*book.Sheet, error)
	// GetSheet returns a Sheet of the specified sheet name.
	GetSheet(name string) (*book.Sheet, error)
}

func New(filename string, setters ...Option) Importer {
	opts := parseOptions(setters...)
	fmt := format.Ext2Format(filepath.Ext(filename))
	switch fmt {
	case format.Excel:
		return NewExcelImporter(filename, opts.Sheets, opts.Parser)
	case format.CSV:
		return NewCSVImporter(filename, opts.Sheets, opts.Parser)
	case format.XML:
		return NewXMLImporter(filename, opts.Sheets, opts.Header)
	default:
		return nil
	}
}

func ParseCSVBookName(filename string) string {
	bookName, _ := parseCSVFilenamePattern(filename)
	return bookName
}
