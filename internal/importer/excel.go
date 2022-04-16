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

// topN: 0 means read all rows
func NewExcelImporter(filename string, sheetNames []string, parser book.SheetParser, topN uint) (*ExcelImporter, error) {
	book, err := parseExcelBook(filename, sheetNames, parser, topN)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to parse excel book")
	}

	return &ExcelImporter{
		Book: book,
	}, nil
}

func parseExcelBook(filename string, sheetNames []string, parser book.SheetParser, topN uint) (*book.Book, error) {
	book, err := readExcelBook(filename, parser, topN)
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

func readExcelBook(filename string, parser book.SheetParser, topN uint) (*book.Book, error) {
	file, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file %s", filename)
	}

	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(bookName, filename, parser)
	for _, sheetName := range file.GetSheetList() {
		rows, err := readExcelSheetRows(file, sheetName, topN)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get rows of sheet: %s#%s", filename, sheetName)
		}
		sheet := book.NewSheet(sheetName, rows)
		newBook.AddSheet(sheet)
	}

	return newBook, nil
}

func readExcelSheetRows(f *excelize.File, sheetName string, topN uint) (rows [][]string, err error) {
	if topN == 0 {
		// read all rows
		rows, err := f.GetRows(sheetName)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get all rows of sheet: %s#%s", f.Path, sheetName)
		}
		return rows, nil
	}

	// read top N rows
	excelRows, err := f.Rows(sheetName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get topN(%d) rows of sheet: %s#%s", topN, f.Path, sheetName)
	}
	var nrow uint
	for excelRows.Next() {
		nrow++
		if nrow > topN {
			break
		}
		row, err := excelRows.Columns()
		if err != nil {
			return nil, errors.Wrapf(err, "read the %-th row failed: %s#%s", nrow, f.Path, sheetName)
		}
		rows = append(rows, row)
	}
	return rows, nil
}
