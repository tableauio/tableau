package importer

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/metasheet"
	"github.com/tableauio/tableau/internal/importer/xlsx"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"github.com/xuri/excelize/v2"
)

var ErrSheetNotFound = errors.New("sheet not found")

type ExcelImporter struct {
	*book.Book
}

type excelRowReader interface {
	SheetNames() []string
	ReadRows(sheetName string, topN uint) ([][]string, error)
}

type rawExcelRowReader struct {
	reader *xlsx.Reader
}

func (r rawExcelRowReader) SheetNames() []string {
	return r.reader.SheetNames()
}

func (r rawExcelRowReader) ReadRows(sheetName string, topN uint) ([][]string, error) {
	rows, err := r.reader.ReadRowsN(sheetName, topN)
	if errors.Is(err, xlsx.ErrSheetNotFound) {
		return nil, ErrSheetNotFound
	}
	return rows, err
}

type excelizeRowReader struct {
	file *excelize.File
}

func (r excelizeRowReader) SheetNames() []string {
	return r.file.GetSheetList()
}

func (r excelizeRowReader) ReadRows(sheetName string, topN uint) ([][]string, error) {
	return readExcelSheetRows(r.file, sheetName, topN, excelize.Options{RawCellValue: true})
}

func NewExcelImporter(ctx context.Context, filename string, setters ...Option) (*ExcelImporter, error) {
	opts := parseOptions(setters...)
	if opts.Mode == Protogen {
		fastImporter, err := newRawExcelImporter(ctx, filename, opts)
		if err == nil {
			return fastImporter, nil
		}
		log.Debugf("raw XLSX reader unavailable for protogen %s, using excelize: %v", filename, err)
	}
	return newExcelizeImporter(ctx, filename, opts)
}

func newRawExcelImporter(ctx context.Context, filename string, opts *Options) (*ExcelImporter, error) {
	reader, err := xlsx.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := reader.Close(); err != nil {
			log.Error(err)
		}
	}()
	return readExcelImporter(ctx, filename, opts, rawExcelRowReader{reader: reader}, true)
}

func newExcelizeImporter(ctx context.Context, filename string, opts *Options) (*ExcelImporter, error) {
	file, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, xerrors.E3002(err)
	}
	defer func() {
		// Close the spreadsheet.
		if err := file.Close(); err != nil {
			log.Error(err)
		}
	}()
	return readExcelImporter(ctx, filename, opts, excelizeRowReader{file: file}, false)
}

func readExcelImporter(ctx context.Context, filename string, opts *Options, reader excelRowReader, filterByMetasheet bool) (*ExcelImporter, error) {
	brOpts := buildExcelBookReaderOptions(filename, reader.SheetNames(), opts.Sheets)

	if opts.Mode == Protogen {
		err := adjustExcelReadOptions(ctx, reader, brOpts, opts.Parser, opts.Cloned, filterByMetasheet)
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to read book: %s", filename)
		}
	}

	loadedBook, err := readExcelBook(ctx, reader, brOpts, opts.Parser)
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to read book: %s", filename)
	}

	if opts.Mode == Protogen {
		if err := loadedBook.ParseMetaAndPurge(); err != nil {
			return nil, xerrors.Wrapf(err, "failed to parse metasheet")
		}
	}

	return &ExcelImporter{
		Book: loadedBook,
	}, nil
}

func adjustExcelReadOptions(ctx context.Context, reader excelRowReader, brOpts *bookReaderOptions, parser book.SheetParser, cloned, filterByMetasheet bool) error {
	if parser != nil && !cloned {
		// Parse the metasheet before data sheets so ordinary schemas need only
		// their header rows. Transposed and special-mode sheets still need all
		// rows because their data contributes to the schema.
		metasheetName := metasheet.FromContext(ctx).Name
		ms, err := readExcelMetasheet(reader, metasheetName)
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
		meta, err := ms.ParseMetasheet(parser)
		if err != nil {
			return xerrors.Wrapf(err, "failed to parse metasheet: %s", metasheetName)
		}

		if filterByMetasheet && len(meta.MetasheetMap) > 0 {
			brOpts.Sheets = filterExcelSheets(brOpts.Sheets, meta.MetasheetMap, metasheetName)
		}
		for _, srOpts := range brOpts.Sheets {
			if srOpts.Name == metasheetName {
				// for metasheet, read all rows
				srOpts.TopN = 0
				continue
			}
			sheetMeta := meta.MetasheetMap[srOpts.Name]
			if sheetMeta == nil || (sheetMeta.Mode == tableaupb.Mode_MODE_DEFAULT && !sheetMeta.Transpose) {
				log.Debugf("sheet %s is in default mode and not transpose, so topN is reset to defaultTopN: %d", srOpts.Name, defaultTopN)
				srOpts.TopN = defaultTopN
			}
		}
	}
	return nil
}

func filterExcelSheets(sheets []*sheetReaderOptions, meta map[string]*internalpb.Metasheet, metasheetName string) []*sheetReaderOptions {
	selected := sheets[:0]
	for _, sheet := range sheets {
		_, listed := meta[sheet.Name]
		if sheet.Name == metasheetName || listed {
			selected = append(selected, sheet)
		}
	}
	return selected
}

func readExcelBook(ctx context.Context, reader excelRowReader, brOpts *bookReaderOptions, parser book.SheetParser) (*book.Book, error) {
	newBook := book.NewBook(ctx, brOpts.Name, brOpts.Filename, parser)
	sheets, err := readExcelSheets(reader, brOpts.Filename, brOpts.Sheets)
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to read excel: %s", brOpts.Filename)
	}
	for _, sheet := range sheets {
		newBook.AddSheet(sheet)
	}
	return newBook, nil
}

// readExcelMetasheet reads all rows of metasheet.
func readExcelMetasheet(reader excelRowReader, sheetName string) (*book.Sheet, error) {
	rows, err := reader.ReadRows(sheetName, 0)
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to get rows of sheet: %s", sheetName)
	}
	return book.NewTableSheet(sheetName, rows), nil
}

func readExcelSheets(reader excelRowReader, filename string, srOpts []*sheetReaderOptions) ([]*book.Sheet, error) {
	var sheets []*book.Sheet
	for _, sheetReader := range srOpts {
		rows, err := reader.ReadRows(sheetReader.Name, sheetReader.TopN)
		if err != nil {
			if errors.Is(err, ErrSheetNotFound) {
				return nil, xerrors.E3001(sheetReader.Name, filename)
			}
			return nil, xerrors.Wrapf(err, "failed to get rows of sheet: %s", sheetReader.Name)
		}
		sheets = append(sheets, book.NewTableSheet(sheetReader.Name, rows))
	}

	return sheets, nil
}

// readExcelSheetRows reads topN rows of specified sheet from excel file.
// NOTE: If topN is 0, then reads all rows.
func readExcelSheetRows(f *excelize.File, sheetName string, topN uint, opts ...excelize.Options) (rows [][]string, err error) {
	if idx, err := f.GetSheetIndex(sheetName); err != nil {
		return nil, xerrors.Wrapf(err, "failed to get sheet index: %s", sheetName)
	} else if idx == -1 {
		return nil, ErrSheetNotFound
	}

	// topN: 0 means read all rows
	if topN == 0 {
		// GetRows fetched all rows with value or formula cells, the continually blank
		// cells in the tail of each row will be skipped.
		rows, err := f.GetRows(sheetName, opts...)
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
	defer func() {
		if closeErr := excelRows.Close(); closeErr != nil && err == nil {
			err = xerrors.Wrapf(closeErr, "failed to close row iterator: %s#%s", f.Path, sheetName)
		}
	}()
	var nrow uint
	for excelRows.Next() {
		nrow++
		if nrow > topN {
			break
		}
		row, err := excelRows.Columns(opts...)
		if err != nil {
			return nil, xerrors.Wrapf(err, "read the %dth row failed: %s#%s", nrow, f.Path, sheetName)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func buildExcelBookReaderOptions(filename string, available, selected []string) *bookReaderOptions {
	brOpts := &bookReaderOptions{
		Name:     strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)),
		Filename: filename,
	}
	for _, sheetName := range available {
		if wantSheet(sheetName, selected) {
			shReaderOpt := &sheetReaderOptions{
				Filename: filename,
				Name:     sheetName,
			}
			brOpts.Sheets = append(brOpts.Sheets, shReaderOpt)
		}
	}
	return brOpts
}
