package xfs

import (
	"path/filepath"
	"strings"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/xerrors"
)

func ParseCSVFilenamePattern(filename string) (bookName, sheetName string, err error) {
	// Recognize pattern: "<BookName>#<SheetName>.csv"
	basename := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	splits := strings.SplitN(basename, "#", 2)
	if len(splits) == 2 {
		return CleanSlashPath(splits[0]), splits[1], nil
	}
	return "", "", xerrors.Errorf("cannot parse the book name and sheet name from filename: %s", filename)
}

func GenCSVBooknamePattern(dir, bookName string) string {
	bookNamePattern := bookName + "#*" + format.CSVExt
	return CleanSlashPath(filepath.Join(dir, bookNamePattern))
}

func ParseCSVBooknamePatternFrom(filename string) (string, error) {
	dir := filepath.Dir(filename)
	bookName, _, err := ParseCSVFilenamePattern(filename)
	if err != nil {
		return "", err
	}
	return GenCSVBooknamePattern(dir, bookName), nil
}
