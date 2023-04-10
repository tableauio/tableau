package importer

import "github.com/tableauio/tableau/internal/importer/book"

type bookReaderOptions struct {
	Name     string // book name without suffix
	Filename string // book filename with path
	Sheets   []*sheetReaderOptions
}

type sheetReaderOptions struct {
	Name     string // sheet name
	Filename string // filename which this sheet belonged to
	TopN     uint
}

func (b *bookReaderOptions) GetMetasheet() *sheetReaderOptions {
	for _, sheet := range b.Sheets {
		if sheet.Name == book.MetasheetName {
			return sheet
		}
	}
	return nil
}

func NeedSheet(sheetName string, wantSheetNames []string) bool {
	if len(wantSheetNames) == 0 {
		// read all sheets if wantSheetNames not set.
		return true
	}
	for _, name := range wantSheetNames {
		if name == sheetName {
			return true
		}
	}
	return false
}
