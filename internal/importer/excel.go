package importer

import (
	errs "errors"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/xuri/excelize/v2"
)

var ErrSheetNotFound = errs.New("sheet not found")

type ExcelImporter struct {
	*book.Book
}

func NewExcelImporter(filename string, sheetNames []string, parser book.SheetParser, mode ImporterMode, cloned bool) (*ExcelImporter, error) {
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

	var shReaderOpts []*sheetReaderOptions
	// read all sheets if sheetNames not set.
	if len(sheetNames) == 0 {
		for _, sheetName := range file.GetSheetList() {
			shReaderOpts = append(shReaderOpts, &sheetReaderOptions{Name: sheetName})
		}
	} else {
		for _, sheetName := range sheetNames {
			shReaderOpts = append(shReaderOpts, &sheetReaderOptions{Name: sheetName})
		}
	}

	if mode == Protogen {
		err := adjustTopN(file, parser, cloned, shReaderOpts)
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to read book: %s", filename)
		}
	}

	book, err := readExcelBook(filename, file, shReaderOpts, parser)
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

func adjustTopN(file *excelize.File, parser book.SheetParser, cloned bool, shReaderOpts []*sheetReaderOptions) error {
	if parser != nil && !cloned {
		// parse metasheet, and change topN to 0 if any sheet is transpose or not default mode.
		metasheet, err := readExcelSheet(file, book.MetasheetName, 0)
		if err != nil {
			if errors.Is(err, ErrSheetNotFound) {
				log.Debugf("metasheet not found, use default TopN: %d", defaultTopN)
				for _, shReaderOpts := range shReaderOpts {
					shReaderOpts.TopN = defaultTopN
				}
				return nil
			}
			return err
		}
		meta, err := book.ParseMetasheet(metasheet, parser)
		if err != nil {
			return errors.WithMessagef(err, "failed to parse metasheet: %s", book.MetasheetName)
		}

		for _, shReaderOpts := range shReaderOpts {
			metasheet := meta.MetasheetMap[shReaderOpts.Name]
			if metasheet == nil || (metasheet.Mode == tableaupb.Mode_MODE_DEFAULT && !metasheet.Transpose) {
				log.Debugf("sheet %s is in default mode and not transpose, so topN is reset to defaultTopN: %d", defaultTopN)
				shReaderOpts.TopN = defaultTopN
			}
		}
	}
	return nil
}

func readExcelBook(filename string, file *excelize.File, shReaderOpts []*sheetReaderOptions, parser book.SheetParser) (*book.Book, error) {
	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(bookName, filename, parser)
	sheets, err := readExcelSheets(file, shReaderOpts)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to read excel sheets: %s, %v", filename, shReaderOpts)
	}
	for _, sheet := range sheets {
		newBook.AddSheet(sheet)
	}
	return newBook, nil
}

func readExcelSheet(file *excelize.File, sheetName string, topN uint) (*book.Sheet, error) {
	shReaderOpts := &sheetReaderOptions{Name: sheetName, TopN: topN}
	sheets, err := readExcelSheets(file, []*sheetReaderOptions{shReaderOpts})
	if err != nil {
		return nil, err
	}
	return sheets[0], nil
}

func readExcelSheets(file *excelize.File, shReaderOpts []*sheetReaderOptions) ([]*book.Sheet, error) {
	var sheets []*book.Sheet
	for _, sheetReader := range shReaderOpts {
		rows, err := readExcelSheetRows(file, sheetReader.Name, sheetReader.TopN)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get rows of sheet: %s", sheetReader.Name)
		}
		sheets = append(sheets, book.NewSheet(sheetReader.Name, rows))
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
