package importer

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/atom"
)

type CSVImporter struct {
	filename string
	sheet    *Sheet

	bookName  string
	sheetName string
}

func NewCSVImporter(filename string) *CSVImporter {
	ext := filepath.Ext(filename)
	basename := strings.TrimSuffix(filepath.Base(filename), ext)
	bookName := basename
	sheetName := basename
	// Recognize pattern: "<BookName>#<SheetName>.csv"
	if index := strings.Index(basename, "#"); index != -1 {
		bookName = basename[:index]
		if index+1 >= len(basename) {
			atom.Log.Panicf(`invalid csv name pattern: %s, should comply to "<BookName>#<sheetName>.csv"`, filename)
		}
		sheetName = basename[index+1:]
	}
	return &CSVImporter{
		filename:  filename,
		bookName:  bookName,
		sheetName: sheetName,
	}
}

func (x *CSVImporter) GetSheets() ([]*Sheet, error) {
	if x.sheet == nil {
		if err := x.parse(); err != nil {
			return nil, errors.WithMessagef(err, "failed to parse %s", x.filename)
		}
	}

	sheet, err := x.GetSheet(x.filename)
	if err != nil {
		return nil, errors.WithMessagef(err, "get sheet failed: %s", x.filename)
	}
	return []*Sheet{sheet}, nil
}

// GetSheet returns a Sheet of the specified sheet name.
func (x *CSVImporter) GetSheet(name string) (*Sheet, error) {
	if x.sheet == nil {
		if err := x.parse(); err != nil {
			return nil, errors.WithMessagef(err, "failed to parse %s", x.filename)
		}
	}
	// TODO: support multi sheets.
	return x.sheet, nil
}

func (x *CSVImporter) parse() error {
	f, err := os.Open(x.filename)
	if err != nil {
		return errors.Wrapf(err, "failed to open file %s", x.filename)
	}
	defer f.Close()

	r := csv.NewReader(f)
	// If FieldsPerRecord is negative, records may have a variable number of fields.
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return errors.Wrapf(err, "failed to read file %s", x.filename)
	}

	// NOTE: For CSV, sheet name is the same as filename.
	x.sheet = NewSheet(x.sheetName, records)
	return nil
}

func (x *CSVImporter) ExportExcel() error {
	dir := filepath.Dir(x.filename)
	path := filepath.Join(dir, x.bookName) + ".xlsx"
	file, err := OpenExcel(path, x.sheetName)
	if err != nil {
		return errors.WithMessagef(err, "failed to open file %s", x.filename)
	}

	sheets, err := x.GetSheets()
	if err != nil {
		return errors.WithMessagef(err, "failed to get sheets: %s", x.filename)
	}
	for _, sheet := range sheets {
		if err := sheet.ExportExcel(file); err != nil {
			return errors.WithMessagef(err, "export sheet %s to excel failed", sheet.Name)
		}
	}
	// Save spreadsheet by the given path.
	if err := file.SaveAs(path); err != nil {
		return errors.WithMessagef(err, "failed to save file %s", path)
	}
	return nil
}
