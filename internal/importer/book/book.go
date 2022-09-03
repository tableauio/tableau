package book

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
)

type Book struct {
	name       string            // book name without suffix
	filename   string            // book filename
	sheets     map[string]*Sheet // sheet name -> sheet
	sheetNames []string          // ordered sheet names

	meta       *tableaupb.Metabook
	metaParser SheetParser
}

func NewBook(bookName, filename string, parser SheetParser) *Book {
	return &Book{
		name:     bookName,
		filename: filename,
		sheets:   make(map[string]*Sheet),
		meta: &tableaupb.Metabook{
			MetasheetMap: make(map[string]*tableaupb.Metasheet),
		},
		metaParser: parser,
	}
}

// Filename returns this book's original filename.
func (b *Book) Filename() string {
	return b.filename
}

// BookNames returns this book's name.
func (b *Book) BookName() string {
	return b.name
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
	log.Debugf("delete sheet: %s", sheetName)
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

	// NOTE(wenchgy): must deep-copy the sheetNames, as we will delete 
	// the elements when looping the slice at the same time.
	deeplyCopiedSheetNames := make([]string, len(b.sheetNames))
	copy(deeplyCopiedSheetNames, b.sheetNames)

	for _, sheetName := range deeplyCopiedSheetNames {
		if !sheetNameMap[sheetName] {
			b.DelSheet(sheetName)
		}
	}
}

// Clear clears all sheets in the book.
func (b *Book) Clear() {
	b.sheets = make(map[string]*Sheet)
	b.sheetNames = nil
}

func (b *Book) ParseMeta() error {
	sheet := b.GetSheet(MetasheetName)
	if sheet == nil {
		log.Debugf("sheet %s not found in book %s", MetasheetName, b.Filename())
		b.Clear()
		return nil
	}

	if sheet.MaxRow <= 1 {
		// need all sheets except the metasheet "@TABLEAU"
		for _, sheet := range b.GetSheets() {
			if sheet.Name != MetasheetName {
				b.meta.MetasheetMap[sheet.Name] = &tableaupb.Metasheet{
					Sheet: sheet.Name,
				}
			}
		}
	} else {
		if err := b.metaParser.Parse(b.meta, sheet); err != nil {
			return errors.WithMessagef(err, "failed to parse sheet: %s#%s", b.Filename(), MetasheetName)
		}
	}

	log.Debugf("%s#%s: %+v", b.Filename(), MetasheetName, b.meta)

	var keepedSheetNames []string
	for sheetName, sheetMeta := range b.meta.MetasheetMap {
		sheet := b.GetSheet(sheetName)
		if sheet == nil {
			return errors.Errorf("sheet %s not found in book %s", sheetName, b.Filename())
		}
		keepedSheetNames = append(keepedSheetNames, sheetName)
		sheet.Meta = sheetMeta
	}
	// NOTE: only keep the sheets that are specified in meta
	b.Squeeze(keepedSheetNames)
	log.Debugf("squeezed: %s#%s: %+v", b.Filename(), MetasheetName, keepedSheetNames)
	for sheetName := range b.sheets {
		log.Debugf("sheet: %s", sheetName)
	}
	return nil
}

func (b *Book) ExportExcel() error {
	dir := filepath.Dir(b.filename)
	filename := filepath.Join(dir, b.name+format.ExcelExt)
	if len(b.sheetNames) == 0 {
		return nil
	}
	file, err := excel.Open(filename, b.sheetNames[0])
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

func (b *Book) ExportCSV() error {
	dir := filepath.Dir(b.filename)
	basename := filepath.Join(dir, b.name)
	if len(b.sheetNames) == 0 {
		return nil
	}
	for _, sheet := range b.GetSheets() {
		path := fmt.Sprintf("%s#%s%s", basename, sheet.Name, format.CSVExt)
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
