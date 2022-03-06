package importer

import (
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/xuri/excelize/v2"
)

type ExcelImporter struct {
	*book.Book
}

func NewExcelImporter(filename string, sheetNames []string, parser book.SheetParser) (*ExcelImporter, error) {
	book, err := parseExcelBook(filename, sheetNames, parser)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to parse excel book")
	}

	return &ExcelImporter{
		Book: book,
	}, nil
}

func parseExcelBook(filename string, sheetNames []string, parser book.SheetParser) (*book.Book, error) {
	book, err := readExcelBook(filename, parser)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to read book: %s", filename)
	}

	if parser != nil {
		if err := book.ParseMeta(); err != nil {
			return nil, errors.WithMessage(err, "failed to parse metasheet")
		}
	}

	if sheetNames != nil {
		book.Squeeze(sheetNames)
	}
	return book, nil
}

func readExcelBook(filename string, parser book.SheetParser) (*book.Book, error) {
	file, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file %s", filename)
	}

	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(bookName, filename, parser)
	for _, sheetName := range file.GetSheetList() {
		rows, err := file.GetRows(sheetName)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get rows of sheet: %s#%s", filename, sheetName)
		}
		sheet := book.NewSheet(sheetName, rows)
		newBook.AddSheet(sheet)
	}

	return newBook, nil
}
