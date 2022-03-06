package importer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/emirpasic/gods/maps/treemap"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/xuri/excelize/v2"
)

// MetaSheetName defines the meta data of each worksheet.
const MetaSheetName = "@TABLEAU"

type ExcelImporter struct {
	filename string

	sheetMap         map[string]*book.Sheet // sheet name -> sheet
	sheetNames       []string          // ordered sheet names
	includeMetaSheet bool

	Meta       *tableaupb.WorkbookMeta
	metaParser book.SheetParser
}

// TODO: options
func NewExcelImporter(filename string, sheetNames []string, parser book.SheetParser, includeMetaSheet bool) *ExcelImporter {
	return &ExcelImporter{
		filename:         filename,
		sheetNames:       sheetNames,
		includeMetaSheet: includeMetaSheet,
		metaParser:       parser,
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
	if x.sheetMap == nil {
		if err := x.parse(); err != nil {
			return nil, errors.WithMessagef(err, "failed to parse %s", x.filename)
		}
	}

	sheets := []*book.Sheet{}
	for _, name := range x.sheetNames {
		sheet, err := x.GetSheet(name)
		if err != nil {
			return nil, errors.WithMessagef(err, "get sheet failed: %s", name)
		}
		sheets = append(sheets, sheet)
	}
	return sheets, nil
}

// GetSheet returns a Sheet of the specified sheet name.
func (x *ExcelImporter) GetSheet(name string) (*book.Sheet, error) {
	if x.sheetMap == nil {
		if err := x.parse(); err != nil {
			return nil, errors.WithMessagef(err, "failed to parse %s", x.filename)
		}
	}

	sheet, ok := x.sheetMap[name]
	if !ok {
		return nil, errors.Errorf("sheet %s not found", name)
	}
	return sheet, nil
}

func (x *ExcelImporter) parse() error {
	x.sheetMap = make(map[string]*book.Sheet)
	file, err := excelize.OpenFile(x.filename)
	if err != nil {
		return errors.Wrapf(err, "failed to open file %s", x.filename)
	}

	if err := x.parseWorkbookMeta(file); err != nil {
		return errors.Wrapf(err, "failed to parse workbook meta: %s", MetaSheetName)
	}

	if err := x.collectSheetsInOrder(file); err != nil {
		return errors.WithMessagef(err, "failed to collectSheetsInOrder: %s", x.filename)
	}

	for _, sheetName := range x.sheetNames {
		s, err := x.parseSheet(file, sheetName)
		if err != nil {
			return errors.WithMessagef(err, "failed to parse sheet: %s#%s", x.filename, sheetName)
		}
		x.sheetMap[sheetName] = s
	}
	return nil
}
func (x *ExcelImporter) NeedParseMeta() bool {
	return x.metaParser != nil
}

func (x *ExcelImporter) parseWorkbookMeta(file *excelize.File) error {
	if !x.NeedParseMeta() {
		// atom.Log.Debugf("skip parsing workbook meta: %s", x.filename)
		return nil
	}

	if file.GetSheetIndex(MetaSheetName) == -1 {
		atom.Log.Debugf("workbook %s has no sheet named %s", x.filename, MetaSheetName)
		return nil
	}

	sheet, err := x.parseSheet(file, MetaSheetName)
	if err != nil {
		return errors.WithMessagef(err, "failed to parse sheet: %s#%s", x.filename, MetaSheetName)
	}

	if sheet.MaxRow <= 1 {
		for _, sheetName := range file.GetSheetList() {
			x.Meta.SheetMetaMap[sheetName] = &tableaupb.SheetMeta{
				Sheet: sheetName,
			}
		}
		return nil
	}
	if err := x.metaParser.Parse(x.Meta, sheet); err != nil {
		return errors.WithMessagef(err, "failed to parse sheet: %s#%s", x.filename, MetaSheetName)
	}

	atom.Log.Debugf("%s#%s: %+v", x.filename, MetaSheetName, x.Meta)
	return nil
}

func (x *ExcelImporter) parseSheet(file *excelize.File, sheetName string) (*book.Sheet, error) {
	rows, err := file.GetRows(sheetName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get rows of sheet: %s#%s", x.filename, sheetName)
	}
	sheet := book.NewSheet(sheetName, rows)

	if x.NeedParseMeta() {
		sheet.Meta = x.Meta.SheetMetaMap[sheetName]
	}
	return sheet, nil
}

func (x *ExcelImporter) collectSheetsInOrder(file *excelize.File) error {
	sortedMap := treemap.NewWithIntComparator()
	if x.NeedParseMeta() {
		for sheetName := range x.Meta.SheetMetaMap {
			index := file.GetSheetIndex(sheetName)
			if index == -1 {
				return errors.Errorf("sheet %s not found in workbook %s", sheetName, x.filename)
			}
			sortedMap.Put(index, sheetName)
		}
	} else {
		// Import all sheets except `@TABLEAU` if x.sheetNames is empty.
		if x.sheetNames == nil {
			for index, name := range file.GetSheetMap() {
				sortedMap.Put(index, name)
			}
		}

		for _, name := range x.sheetNames {
			index := file.GetSheetIndex(name)
			if index == -1 {
				return errors.Errorf("sheet %s not found in workbook %s", name, x.filename)
			}
			sortedMap.Put(index, name)
		}
	}

	// Clear before re-assign.
	x.sheetNames = nil
	for _, val := range sortedMap.Values() {
		sheetName := val.(string)
		if sheetName != MetaSheetName || (x.includeMetaSheet && sheetName == MetaSheetName) {
			// exclude meta sheet
			x.sheetNames = append(x.sheetNames, sheetName)
		}
	}
	return nil
}

func (x *ExcelImporter) ExportCSV() error {
	ext := filepath.Ext(x.filename)
	basename := strings.TrimSuffix(x.filename, ext)
	sheets, err := x.GetSheets()
	if err != nil {
		return errors.WithMessagef(err, "failed to get sheets: %s", x.filename)
	}
	for _, sheet := range sheets {
		path := fmt.Sprintf("%s#%s.csv", basename, sheet.Name)
		f, err := os.Create(path)
		if err != nil {
			return errors.Wrapf(err, "failed to create csv file: %s", path)
		}
		defer f.Close()

		if err := sheet.ExportCSV(f); err != nil {
			return errors.WithMessagef(err, "export sheet %s to excel failed", sheet.Name)
		}
	}
	return nil
}
