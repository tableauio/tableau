package importer

import (
	errs "errors"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/log"
	"github.com/xuri/excelize/v2"
)

var ErrSheetNotFound = errs.New("sheet not found")

type ExcelImporter struct {
	*book.Book
}

func NewExcelImporter(filename string, sheetNames []string, parser book.SheetParser, mode ImporterMode, merged bool) (*ExcelImporter, error) {
	file, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file %s", filename)
	}
	defer func() {
		// Close the spreadsheet.
		if err := file.Close(); err != nil {
			log.Error(err)
		}
	}()

	var topN uint
	if mode == Protogen {
		n, err := adjustTopN(file, parser, merged)
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to read book: %s", filename)
		}
		topN = n
	}

	book, err := readExcelBook(filename, file, sheetNames, parser, topN)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to read book: %s", filename)
	}

	if parser != nil {
		if err := book.ParseMetaAndPurge(); err != nil {
			return nil, errors.WithMessage(err, "failed to parse metasheet")
		}
	}

	return &ExcelImporter{
		Book: book,
	}, nil
}

func adjustTopN(file *excelize.File, parser book.SheetParser, merged bool) (uint, error) {
	if parser != nil && !merged {
		// parse metasheet, and change topN to 0 if any sheet is transpose
		metasheet, err := readExcelSheet(file, book.MetasheetName, 0)
		if err != nil {
			if errors.Is(err, ErrSheetNotFound) {
				log.Debugf("sheet not found, use default TopN: %d", defaultTopN)
				return defaultTopN, nil
			}
			return 0, err
		}
		meta, err := book.ParseMetasheet(metasheet, parser)
		if err != nil {
			return 0, errors.WithMessagef(err, "failed to parse metasheet: %s", book.MetasheetName)
		}
		if len(meta.MetasheetMap) != 0 {
			for name, sheet := range meta.MetasheetMap {
				if sheet.Transpose {
					log.Debugf("sheet %s is transpose, so topN is reset to 0", name)
					return 0, nil
				}
			}
		}
	}
	return defaultTopN, nil
}

func readExcelBook(filename string, file *excelize.File, sheetNames []string, parser book.SheetParser, topN uint) (*book.Book, error) {
	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(bookName, filename, parser)
	sheets, err := readExcelSheets(file, sheetNames, topN)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to read excel sheets: %s#%v", filename, sheetNames)
	}
	for _, sheet := range sheets {
		newBook.AddSheet(sheet)
	}
	return newBook, nil
}

func readExcelSheet(file *excelize.File, sheetName string, topN uint) (*book.Sheet, error) {
	sheets, err := readExcelSheets(file, []string{sheetName}, topN)
	if err != nil {
		return nil, err
	}
	return sheets[0], nil
}

func readExcelSheets(file *excelize.File, sheetNames []string, topN uint) ([]*book.Sheet, error) {
	// read all sheets if sheetNames not set.
	if len(sheetNames) == 0 {
		sheetNames = file.GetSheetList()
	}

	var sheets []*book.Sheet
	for _, sheetName := range sheetNames {
		rows, err := readExcelSheetRows(file, sheetName, topN)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get rows of sheet: %s", sheetName)
		}
		sheets = append(sheets, book.NewSheet(sheetName, rows))
	}

	return sheets, nil
}

func readExcelSheetRows(f *excelize.File, sheetName string, topN uint) (rows [][]string, err error) {
	if f.GetSheetIndex(sheetName) == -1 {
		return nil, ErrSheetNotFound
	}

	// topN: 0 means read all rows
	if topN == 0 {
		// GetRows fetched all rows with value or formula cells, the continually blank
		// cells in the tail of each row will be skipped.
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
			return nil, errors.Wrapf(err, "read the %dth row failed: %s#%s", nrow, f.Path, sheetName)
		}
		rows = append(rows, row)
	}
	if sheetName == book.MetasheetName {
		log.Debugf("read %d rows (topN:%d) from sheet: %s#%s", len(rows), topN, f.Path, sheetName)
	}
	return rows, nil
}
