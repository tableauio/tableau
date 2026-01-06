package importer

import (
	"path/filepath"

	"github.com/tableauio/tableau/log"
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

// wantSheet checks whether the sheet name matches the wantSheetNames which are
// checked by [filepath.Match].
func wantSheet(sheetName string, wantSheetNames []string) bool {
	if len(wantSheetNames) == 0 {
		// read all sheets if wantSheetNames not set.
		return true
	}
	for _, wantSheetName := range wantSheetNames {
		matched, err := filepath.Match(wantSheetName, sheetName)
		if err != nil {
			// As the [filepath.Match] says: "The only possible returned error
			// is ErrBadPattern, when pattern is malformed." So we just log an
			// error for debug and return false.
			log.Errorf("sheet name pattern is malformed: %s, %s, err: %s", sheetName, wantSheetName, err)
			return false
		}
		if matched {
			return true
		}
	}
	return false
}
