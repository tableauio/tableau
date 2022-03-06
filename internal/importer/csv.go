package importer

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"

	"github.com/emirpasic/gods/sets/treeset"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/proto/tableaupb"
)

// CSVImporter recognizes pattern: "<BookName>#<SheetName>.csv"
type CSVImporter struct {
	book               *book.Book
	filename           string
	selectedSheetNames []string // selected sheet names

	Meta       *tableaupb.WorkbookMeta
	metaParser book.SheetParser
}

func NewCSVImporter(filename string, sheetNames []string, parser book.SheetParser) *CSVImporter {
	return &CSVImporter{
		filename:           filename,
		selectedSheetNames: sheetNames,
		metaParser:         parser,
		Meta: &tableaupb.WorkbookMeta{
			SheetMetaMap: make(map[string]*tableaupb.SheetMeta),
		},
	}
}

func (x *CSVImporter) BookName() string {
	bookName, _ := parseCSVFilenamePattern(x.filename)
	return bookName
}

func (x *CSVImporter) Filename() string {
	bookName, _ := parseCSVFilenamePattern(x.filename)
	// convert filename "<BookName>#<SheetName>.csv" to "<BookName>#*.csv"
	dir := filepath.Dir(x.filename)
	return genCSVBookFilenamePattern(dir, bookName)
}

func (x *CSVImporter) GetSheets() ([]*book.Sheet, error) {
	if x.book == nil {
		if err := x.parseBook(); err != nil {
			return nil, errors.WithMessagef(err, "failed to parse csv book: %s", x.Filename())
		}
	}
	return x.book.GetSheets(), nil
}

// GetSheet returns a Sheet of the specified sheet name.
func (x *CSVImporter) GetSheet(name string) (*book.Sheet, error) {
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

func (x *CSVImporter) parseBook() error {
	bookName, _ := parseCSVFilenamePattern(x.filename)
	if bookName == "" {
		x.book = book.NewBook(bookName)
		return nil
	}

	book, err := readCSVBook(filepath.Dir(x.filename), bookName)
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

func genCSVBookFilenamePattern(dir, bookName string) string {
	bookNamePattern := bookName + "#*.csv"
	return filepath.Join(dir, bookNamePattern)
}

func parseCSVFilenamePattern(filename string) (bookName, sheetName string) {
	// Recognize pattern: "<BookName>#<SheetName>.csv"
	basename := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	if index := strings.Index(basename, "#"); index != -1 {
		if index+1 < len(basename) {
			bookName = basename[:index]
			sheetName = basename[index+1:]
		}
	}
	return
}

func readCSV(filename string) ([][]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file: %s", filename)
	}
	defer f.Close()

	r := csv.NewReader(f)
	// If FieldsPerRecord is negative, records may have a variable number of fields.
	r.FieldsPerRecord = -1
	return r.ReadAll()
}

func readCSVBook(dir, bookName string) (*book.Book, error) {
	globFilename := genCSVBookFilenamePattern(dir, bookName)
	matches, err := filepath.Glob(globFilename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to glob %s", globFilename)
	}

	// NOTE: keep the order of sheets
	set := treeset.NewWithStringComparator()
	for _, filename := range matches {
		set.Add(filename)
	}

	newBook := book.NewBook(bookName)
	for _, val := range set.Values() {
		filename := val.(string)
		_, sheetName := parseCSVFilenamePattern(filename)
		if sheetName == "" {
			return nil, errors.Errorf("cannot parse the sheet name from filename: %s", filename)
		}
		records, err := readCSV(filename)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read CSV file: %s", filename)
		}
		sheet := book.NewSheet(sheetName, records)
		newBook.AddSheet(sheet)
	}

	return newBook, nil
}

func (x *CSVImporter) parseWorkbookMeta(book *book.Book) error {
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

func (x *CSVImporter) needParseMeta() bool {
	return x.metaParser != nil
}

func (x *CSVImporter) ExportExcel() error {
	if x.book == nil {
		if err := x.parseBook(); err != nil {
			return errors.WithMessagef(err, "failed to parse %s", x.filename)
		}
	}
	dir := filepath.Dir(x.filename)
	return x.book.ExportExcel(dir)
}
