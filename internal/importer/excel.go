package importer

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"github.com/xuri/excelize/v2"
)

var ErrSheetNotFound = errors.New("sheet not found")

type ExcelImporter struct {
	*book.Book
}

func NewExcelImporter(filename string, sheetNames []string, parser book.SheetParser, mode ImporterMode, cloned bool) (*ExcelImporter, error) {
	file, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to open file %s", filename)
	}
	defer func() {
		// Close the spreadsheet.
		if err := file.Close(); err != nil {
			log.Error(err)
		}
	}()

	brOpts, err := parseExcelBookReaderOptions(filename, file, sheetNames)
	if err != nil {
		return nil, err
	}

	if mode == Protogen {
		err := adjustExcelTopN(file, brOpts, parser, cloned)
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to read book: %s", filename)
		}
	}

	book, err := readExcelBook(file, brOpts, parser)
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to read book: %s", filename)
	}

	if mode == Protogen {
		if err := book.ParseMetaAndPurge(); err != nil {
			return nil, xerrors.Wrapf(err, "failed to parse metasheet")
		}
	}

	return &ExcelImporter{
		Book: book,
	}, nil
}

func adjustExcelTopN(file *excelize.File, brOpts *bookReaderOptions, parser book.SheetParser, cloned bool) error {
	if parser != nil && !cloned {
		// parse metasheet, and change topN to 0 if any sheet is transpose or not default mode.
		metasheet, err := readExcelMetasheet(file)
		if err != nil {
			if errors.Is(err, ErrSheetNotFound) {
				log.Debugf("metasheet not found, use default TopN: %d", defaultTopN)
				for _, srOpts := range brOpts.Sheets {
					srOpts.TopN = defaultTopN
				}
				return nil
			}
			return err
		}
		meta, err := metasheet.ParseMetasheet(parser)
		if err != nil {
			return xerrors.Wrapf(err, "failed to parse metasheet: %s", book.MetasheetName)
		}

		for _, srOpts := range brOpts.Sheets {
			if srOpts.Name == book.MetasheetName {
				// for metasheet, read all rows
				srOpts.TopN = 0
				continue
			}
			metasheet := meta.MetasheetMap[srOpts.Name]
			if metasheet == nil || (metasheet.Mode == tableaupb.Mode_MODE_DEFAULT && !metasheet.Transpose) {
				log.Debugf("sheet %s is in default mode and not transpose, so topN is reset to defaultTopN: %d", srOpts.Name, defaultTopN)
				srOpts.TopN = defaultTopN
			}
		}
	}
	return nil
}

func readExcelBook(file *excelize.File, brOpts *bookReaderOptions, parser book.SheetParser) (*book.Book, error) {
	newBook := book.NewBook(brOpts.Name, brOpts.Filename, parser)
	sheets, err := readExcelSheets(file, brOpts.Sheets)
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to read excel: %s", brOpts.Filename)
	}
	for _, sheet := range sheets {
		newBook.AddSheet(sheet)
	}
	return newBook, nil
}

// readExcelMetasheet reads all rows of metasheet.
func readExcelMetasheet(file *excelize.File) (*book.Sheet, error) {
	sheetName := book.MetasheetName
	rows, err := readExcelSheetRows(file, sheetName, 0)
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to get rows of sheet: %s", sheetName)
	}
	return book.NewTableSheet(sheetName, rows, 0), nil
}

func readExcelSheets(file *excelize.File, srOpts []*sheetReaderOptions) ([]*book.Sheet, error) {
	var sheets []*book.Sheet
	for _, sheetReader := range srOpts {
		rows, err := readExcelSheetRows(file, sheetReader.Name, sheetReader.TopN)
		if err != nil {
			if errors.Is(err, ErrSheetNotFound) {
				return nil, xerrors.E3001(sheetReader.Name, file.Path)
			}
			return nil, xerrors.Wrapf(err, "failed to get rows of sheet: %s", sheetReader.Name)
		}
		sheets = append(sheets, book.NewTableSheet(sheetReader.Name, rows, 0))
	}

	return sheets, nil
}

// readExcelSheetRows reads topN rows of specified sheet from excel file.
// NOTE: If topN is 0, then reads all rows.
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
			return nil, xerrors.Wrapf(err, "failed to get all rows of sheet: %s#%s", f.Path, sheetName)
		}
		return rows, nil
	}

	// read top N rows
	excelRows, err := f.Rows(sheetName)
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to get topN(%d) rows of sheet: %s#%s", topN, f.Path, sheetName)
	}
	var nrow uint
	for excelRows.Next() {
		nrow++
		if nrow > topN {
			break
		}
		row, err := excelRows.Columns()
		if err != nil {
			return nil, xerrors.Wrapf(err, "read the %dth row failed: %s#%s", nrow, f.Path, sheetName)
		}
		rows = append(rows, row)
	}
	if sheetName == book.MetasheetName {
		log.Debugf("read %d rows (topN:%d) from sheet: %s#%s", len(rows), topN, f.Path, sheetName)
	}
	return rows, nil
}

func parseExcelBookReaderOptions(filename string, file *excelize.File, sheetNames []string) (*bookReaderOptions, error) {
	brOpts := &bookReaderOptions{
		Name:     strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)),
		Filename: filename,
	}
	for _, sheetName := range file.GetSheetList() {
		if NeedSheet(sheetName, sheetNames) {
			shReaderOpt := &sheetReaderOptions{
				Filename: filename,
				Name:     sheetName,
			}
			brOpts.Sheets = append(brOpts.Sheets, shReaderOpt)
		}
	}
	return brOpts, nil
}
