package book

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"github.com/tableauio/tableau/xerrors"
)

type Book struct {
	name       string            // book name without suffix
	filename   string            // book filename
	sheets     map[string]*Sheet // sheet name -> sheet
	sheetNames []string          // ordered sheet names

	meta       *internalpb.Metabook
	metaParser SheetParser
}

// NewBook creates a new book.
// Example:
//   - bookName: Test
//   - filename: testdata/Test.xlsx
func NewBook(bookName, filename string, parser SheetParser) *Book {
	return &Book{
		name:     bookName,
		filename: filename,
		sheets:   make(map[string]*Sheet),
		meta: &internalpb.Metabook{
			MetasheetMap: make(map[string]*internalpb.Metasheet),
		},
		metaParser: parser,
	}
}

// Filename returns this book's original filename.
func (b *Book) Filename() string {
	return b.filename
}

// BookName returns this book's name.
func (b *Book) BookName() string {
	return b.name
}

// Format returns this book's format.
func (b *Book) Format() format.Format {
	return format.GetFormat(b.filename)
}

// GetBookOptions creates a new tableaupb.WorkbookOptions
// based on this special sheet(#)'s info.
func (b *Book) GetBookOptions() *tableaupb.WorkbookOptions {
	meta := b.meta.GetMetasheetMap()[BookNameInMetasheet]
	if meta == nil {
		return nil
	}
	return &tableaupb.WorkbookOptions{
		Name:     "", // To be filled by protogen
		Alias:    meta.Alias,
		Namerow:  meta.Namerow,
		Typerow:  meta.Typerow,
		Noterow:  meta.Noterow,
		Datarow:  meta.Datarow,
		Nameline: meta.Nameline,
		Typeline: meta.Typeline,
		Noteline: meta.Noteline,
		Sep:      meta.Sep,
		Subsep:   meta.Subsep,
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

// ParseMetaAndPurge parses metasheet to Metabook and
// purge needless sheets which is not in parsed Metabook.
func (b *Book) ParseMetaAndPurge() (err error) {
	metasheet := b.GetSheet(MetasheetName)
	if metasheet == nil {
		log.Debugf("metasheet %s not found in book %s, maybe it is a to be merged sheet", MetasheetName, b.Filename())
		b.Clear()
		return nil
	}

	b.meta, err = metasheet.ParseMetasheet(b.metaParser)
	if err != nil {
		return xerrors.Wrapf(err, "failed to parse sheet: %s#%s", b.Filename(), MetasheetName)
	}

	if len(b.meta.MetasheetMap) == 0 {
		// need all sheets except the MetasheetName and BookNameInMetasheet
		b.meta.MetasheetMap = make(map[string]*internalpb.Metasheet) // init
		for _, sheet := range b.GetSheets() {
			if sheet.Name != MetasheetName && sheet.Name != BookNameInMetasheet {
				if sheet.Document != nil {
					if sheet.Document.IsMeta() {
						sheetName := sheet.Document.GetDataSheetName()
						b.meta.MetasheetMap[sheetName] = &internalpb.Metasheet{
							Sheet: sheetName,
						}
					}
				} else {
					b.meta.MetasheetMap[sheet.Name] = &internalpb.Metasheet{
						Sheet: sheet.Name,
					}
				}
			}
		}
	}

	log.Debugf("%s#%s: %+v", b.Filename(), MetasheetName, b.meta)

	var reservedSheetNames []string
	for sheetName, sheetMeta := range b.meta.MetasheetMap {
		if sheetName == BookNameInMetasheet {
			continue
		}
		if metasheet.Document != nil {
			sheetName = MetaSign + sheetName
		}
		sheet := b.GetSheet(sheetName)
		if sheet == nil {
			return xerrors.E0001(sheetName, b.Filename())
		}
		reservedSheetNames = append(reservedSheetNames, sheetName)
		sheet.Meta = sheetMeta
	}
	// NOTE: only reserve the sheets that are specified in metasheet
	b.Squeeze(reservedSheetNames)
	log.Debugf("squeezed: %s#%s: %+v", b.Filename(), MetasheetName, reservedSheetNames)
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
		return xerrors.Wrapf(err, "failed to open file %s", filename)
	}

	for _, sheet := range b.GetSheets() {
		if sheet.Table != nil {
			if err := sheet.Table.ExportExcel(file, sheet.Name); err != nil {
				return xerrors.Wrapf(err, "export sheet %s to excel failed", sheet.Name)
			}
		}
	}

	if err := file.SaveAs(filename); err != nil {
		return xerrors.Wrapf(err, "failed to save file %s", filename)
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
			return xerrors.Wrapf(err, "failed to create csv file: %s", path)
		}
		defer f.Close()
		if sheet.Table != nil {
			if err := sheet.Table.ExportCSV(f); err != nil {
				return xerrors.Wrapf(err, "export sheet %s to excel failed", sheet.Name)
			}
		}
	}
	return nil
}

func (b *Book) String() string {
	var str string
	for _, sheet := range b.GetSheets() {
		str += sheet.String() + "\n"
	}
	return str
}
