package importer

import "github.com/tableauio/tableau/internal/importer/book"

type bookReaderOptions struct {
	Filename string // book filename
	Name     string // book name
	Sheets   []*sheetReaderOptions
}

type sheetReaderOptions struct {
	Filename string // filename which this sheet belonged to
	Name     string // sheet name
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
