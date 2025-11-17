package excel

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tableauio/tableau/xerrors"
	"github.com/xuri/excelize/v2"
)

// LetterAxis generate the corresponding column letter position.
//
// NOTE: index is 0-based.
func LetterAxis(index int) string {
	var (
		colCode = ""
		key     = 'A'
		loop    = index / 26
	)
	if loop > 0 {
		colCode += LetterAxis(loop - 1)
	}
	return colCode + string(key+int32(index)%26)
}

// Position generate the position (row, col) (e.g.: A1) in a sheet.
//
// NOTE: row and col are both 0-based.
func Position(row, col int) string {
	return fmt.Sprintf("%s%d", LetterAxis(col), row+1)
}

// Open opens a new excel file. If the file already exists, it will be removed
// firstly.
func Open(filename string, sheetName string) (*excelize.File, error) {
	var wb *excelize.File
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return nil, xerrors.Wrapf(err, "failed to remove old file: %s", filename)
	}
	wb = excelize.NewFile()
	t := time.Now()
	datetime := t.Format(time.RFC3339)
	err := wb.SetDocProps(&excelize.DocProperties{
		Category:       "category",
		ContentStatus:  "Draft",
		Created:        datetime,
		Creator:        "Tableau",
		Description:    "This file was created by Tableau",
		Identifier:     "xlsx",
		Keywords:       "Spreadsheet",
		LastModifiedBy: "Tableau",
		Modified:       datetime,
		Revision:       "0",
		Subject:        "Configuration",
		Title:          filepath.Base(filename),
		Language:       "en-US",
		Version:        "1.0.0",
	})
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to set doc props: %s", filename)
	}
	// The newly created workbook will by default contain a worksheet named `Sheet1`.
	err = wb.SetSheetName("Sheet1", sheetName)
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to set sheet name: %s", sheetName)
	}
	err = wb.SetDefaultFont("Courier")
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to set default font")
	}
	return wb, nil
}
