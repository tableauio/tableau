package importer

import (
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/xuri/excelize/v2"
)

type ExcelImporter struct {
	book     *book.Book
	filename string

	selectedSheetNames []string // selected sheet names

	Meta       *tableaupb.WorkbookMeta
	metaParser book.SheetParser
}

func NewExcelImporter(filename string, sheetNames []string, parser book.SheetParser) *ExcelImporter {
	return &ExcelImporter{
		filename:           filename,
		selectedSheetNames: sheetNames,
		metaParser:         parser,
		Meta: &tableaupb.WorkbookMeta{
			SheetMetaMap: make(map[string]*tableaupb.SheetMeta),
		},
	}
}

func (x *ExcelImporter) BookName() string {
	return strings.TrimSuffix(filepath.Base(x.filename), filepath.Ext(x.filename))
}

func (x *ExcelImporter) Filename() string {
	return x.filename
}

func (x *ExcelImporter) GetSheets() ([]*book.Sheet, error) {
	if x.book == nil {
		if err := x.parseBook(); err != nil {
			return nil, errors.WithMessagef(err, "failed to parse csv book: %s", x.Filename())
		}
	}
	return x.book.GetSheets(), nil
}

// GetSheet returns a Sheet of the specified sheet name.
func (x *ExcelImporter) GetSheet(name string) (*book.Sheet, error) {
	if x.book == nil {
		if err := x.parseBook(); err != nil {
			return nil, errors.WithMessagef(err, "failed to parse %s", x.filename)
		}
	}
	sheet := x.book.GetSheet(name)
	if sheet == nil {
		return nil, errors.Errorf("sheet %s not found", name)
	}
	return sheet, nil
}

func (x *ExcelImporter) parseBook() error {
	book, err := readExcelBook(x.filename)
	if err != nil {
		return errors.WithMessagef(err, "failed to read csv book: %s", x.Filename())
	}

	if x.needParseMeta() {
		if err := x.parseWorkbookMeta(book); err != nil {
			return errors.Wrapf(err, "failed to parse workbook meta: %s", MetaSheetName)
		}
	}

	if x.selectedSheetNames != nil {
		book.Squeeze(x.selectedSheetNames)
	}

	// finally, assign parsed book to importer
	x.book = book
	return nil
}

func (x *ExcelImporter) needParseMeta() bool {
	return x.metaParser != nil
}

func (x *ExcelImporter) parseWorkbookMeta(book *book.Book) error {
	sheet := book.GetSheet(MetaSheetName)
	if sheet == nil {
		atom.Log.Debugf("sheet %s not found in book %s", MetaSheetName, x.Filename())
		return nil
	}

	if sheet.MaxRow <= 1 {
		// need all sheets except the metasheet "@TABLEAU"
		for _, sheet := range book.GetSheets() {
			if sheet.Name != MetaSheetName {
				x.Meta.SheetMetaMap[sheet.Name] = &tableaupb.SheetMeta{
					Sheet: sheet.Name,
				}
			}
		}
	} else {
		if err := x.metaParser.Parse(x.Meta, sheet); err != nil {
			return errors.WithMessagef(err, "failed to parse sheet: %s#%s", x.filename, MetaSheetName)
		}
	}

	atom.Log.Debugf("%s#%s: %+v", x.Filename(), MetaSheetName, x.Meta)

	var keepedSheetNames []string
	for sheetName, sheetMeta := range x.Meta.SheetMetaMap {
		sheet := book.GetSheet(sheetName)
		if sheet == nil {
			return errors.Errorf("sheet %s not found in book %s", sheetName, x.Filename())
		}
		keepedSheetNames = append(keepedSheetNames, sheetName)
		sheet.Meta = sheetMeta
	}
	// NOTE: only keep the sheets that are specified in meta
	book.Squeeze(keepedSheetNames)
	return nil
}

func readExcelBook(filename string) (*book.Book, error) {
	file, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file %s", filename)
	}

	bookName := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	newBook := book.NewBook(bookName)
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

func (x *ExcelImporter) ExportCSV() error {
	if x.book == nil {
		if err := x.parseBook(); err != nil {
			return errors.WithMessagef(err, "failed to parse %s", x.filename)
		}
	}
	dir := filepath.Dir(x.filename)
	return x.book.ExportCSV(dir)
}
