package importer

import (
	"path/filepath"
	"slices"
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

func NeedSheet(sheetName string, wantSheetNames []string) bool {
	if len(wantSheetNames) == 0 {
		// read all sheets if wantSheetNames not set.
		return true
	}
	return slices.ContainsFunc(wantSheetNames, func(wantSheetName string) bool {
		if wantSheetName == sheetName {
			return true
		}
		match, _ := filepath.Match(wantSheetName, sheetName)
		return match
	})
}
