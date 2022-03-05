package importer

import (
	"path/filepath"

	"github.com/pkg/errors"
)

type Book struct {
	Name       string            // book name without suffix
	sheets     map[string]*Sheet // sheet name -> sheet
	sheetNames []string          // ordered sheet names
}

func NewBook(name string) *Book {
	return &Book{
		Name:   name,
		sheets: make(map[string]*Sheet),
	}
}

// BookNames returns this book's name.
func (b *Book) BookName() string {
	return b.Name
}

// AddSheet adds a sheet to the book and keep the sheet order.
func (b *Book) AddSheet(sheet *Sheet) {
	b.sheets[sheet.Name] = sheet
	// delete the sheet name if it exists
	b.delSheetName(sheet.Name)
	b.sheetNames = append(b.sheetNames, sheet.Name)
}

// DelSheet deletes a sheet from the book.
func (b *Book) DelSheet(sheetName string) {
	delete(b.sheets, sheetName)
	b.delSheetName(sheetName)
}

func (b *Book) delSheetName(sheetName string) {
	for i, name := range b.sheetNames {
		if name == sheetName {
			b.sheetNames = append(b.sheetNames[:i], b.sheetNames[i+1:]...)
			break
		}
	}
}

// GetSheets returns all sheets in order in the book.
func (b *Book) GetSheets() []*Sheet {
	var sheets []*Sheet
	for _, sheetName := range b.sheetNames {
		sheet := b.GetSheet(sheetName)
		if sheet == nil {
			panic("sheet not found" + sheetName)
		}
		sheets = append(sheets, sheet)
	}
	return sheets
}

// GetSheet returns a Sheet of the specified sheet name.
func (b *Book) GetSheet(name string) *Sheet {
	return b.sheets[name]
}

// Squeeze keeps only the inputed sheet names and removes other sheets from the book.
func (b *Book) Squeeze(sheetNames []string) {
	sheetNameMap := map[string]bool{} // sheet name -> keep or not (bool)
	for _, sheetName := range sheetNames {
		sheetNameMap[sheetName] = true
	}

	for _, sheetName := range b.sheetNames {
		if !sheetNameMap[sheetName] {
			b.DelSheet(sheetName)
		}
	}
}

func (b *Book) ExportExcel(dir string) error {
	filename := filepath.Join(dir, b.Name+".xlsx")
	if len(b.sheetNames) == 0 {
		return nil
	}
	file, err := OpenExcel(filename, b.sheetNames[0])
	if err != nil {
		return errors.WithMessagef(err, "failed to open file %s", filename)
	}

	for _, sheet := range b.GetSheets() {
		if err := sheet.ExportExcel(file); err != nil {
			return errors.WithMessagef(err, "export sheet %s to excel failed", sheet.Name)
		}
	}

	if err := file.SaveAs(filename); err != nil {
		return errors.WithMessagef(err, "failed to save file %s", filename)
	}
	return nil
}
