package importer

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/metasheet"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
)

var ErrSheetNotFound = errors.New("sheet not found")

type ExcelImporter struct {
	*book.Book
}

func NewExcelImporter(ctx context.Context, filename string, setters ...Option) (*ExcelImporter, error) {
	return importExcel(ctx, filename, parseOptions(setters...))
}

func readExcelImporter(ctx context.Context, filename string, opts *Options, reader excelRowReader, filterByMetasheet bool) (*ExcelImporter, error) {
	brOpts := buildExcelBookReaderOptions(filename, reader.SheetNames(), opts.Sheets)

	var metaSheet *book.Sheet
	if opts.Mode == Protogen {
		var err error
		metaSheet, err = adjustExcelReadOptions(ctx, reader, brOpts, opts.Parser, opts.Cloned, filterByMetasheet)
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to read book: %s", filename)
		}
	}

	loadedBook, err := readExcelBook(ctx, reader, brOpts, opts.Parser, metaSheet)
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

func adjustExcelReadOptions(ctx context.Context, reader excelRowReader, brOpts *bookReaderOptions, parser book.SheetParser, cloned, filterByMetasheet bool) (*book.Sheet, error) {
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
				return nil, nil
			}
			return nil, err
		}
		// The parser may mutate its input. Keep the rows reused by readExcelBook
		// independent of this planning pass.
		parseRows := make([][]string, len(ms.Table.Rows))
		for i, row := range ms.Table.Rows {
			parseRows[i] = append([]string(nil), row...)
		}
		meta, err := book.NewTableSheet(ms.Name, parseRows).ParseMetasheet(parser)
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to parse metasheet: %s", metasheetName)
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
		return ms, nil
	}
	return nil, nil
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

func readExcelBook(ctx context.Context, reader excelRowReader, brOpts *bookReaderOptions, parser book.SheetParser, metaSheet *book.Sheet) (*book.Book, error) {
	newBook := book.NewBook(ctx, brOpts.Name, brOpts.Filename, parser)
	sheets, err := readExcelSheets(reader, brOpts.Filename, brOpts.Sheets, metaSheet)
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

func readExcelSheets(reader excelRowReader, filename string, srOpts []*sheetReaderOptions, metaSheet *book.Sheet) ([]*book.Sheet, error) {
	var sheets []*book.Sheet
	for _, sheetReader := range srOpts {
		if metaSheet != nil && sheetReader.Name == metaSheet.Name {
			sheets = append(sheets, metaSheet)
			continue
		}
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
