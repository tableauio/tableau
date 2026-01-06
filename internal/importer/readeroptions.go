package importer

import (
	"path/filepath"
)

type bookReaderOptions struct {
	Name          string // book name without suffix
	Filename      string // book filename with path
	MetasheetName string
	Sheets        []*sheetReaderOptions
}

type sheetReaderOptions struct {
	Name     string // sheet name
	Filename string // filename which this sheet belonged to
	TopN     uint
}

func (b *bookReaderOptions) GetMetasheet() *sheetReaderOptions {
	for _, sheet := range b.Sheets {
		if sheet.Name == b.MetasheetName {
			return sheet
		}
	}
	return nil
}

func checkSheetWanted(sheetName string, wantSheetNames []string) (bool, error) {
	if len(wantSheetNames) == 0 {
		// read all sheets if wantSheetNames not set.
		return true, nil
	}
	for _, wantSheetName := range wantSheetNames {
		matched, err := filepath.Match(wantSheetName, sheetName)
		if err != nil {
			return false, err
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}
