package fs

import (
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
)

func ParseCSVFilenamePattern(filename string) (bookName, sheetName string, err error) {
	// Recognize pattern: "<BookName>#<SheetName>.csv"
	basename := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	splits := strings.SplitN(basename, "#", 2)
	if len(splits) == 2 {
		return GetCleanSlashPath(splits[0]), splits[1], nil
	}
	return "", "", errors.Errorf("cannot parse the book name and sheet name from filename: %s", filename)
}

func GenCSVBooknamePattern(dir, bookName string) string {
	bookNamePattern := bookName + "#*" + format.CSVExt
	return GetCleanSlashPath(filepath.Join(dir, bookNamePattern))
}

func ParseCSVBooknamePatternFrom(filename string) (string, error) {
	dir := filepath.Dir(filename)
	bookName, _, err := ParseCSVFilenamePattern(filename)
	if err != nil {
		return "", err
	}
	return GenCSVBooknamePattern(dir, bookName), nil
}
