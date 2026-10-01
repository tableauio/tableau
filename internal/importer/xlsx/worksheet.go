package xlsx

import (
	"archive/zip"
	"errors"
	"io"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

// readWorksheetRows parses rows directly from the ZIP stream. After a bounded
// read, draining the entry verifies its checksum without parsing unused cells.
func readWorksheetRows(entry *zip.File, sheetName string, sharedStrings []string, rowLimit uint) ([][]string, error) {
	if _, err := validatedEntrySize(entry); err != nil {
		return nil, err
	}
	source, err := entry.Open()
	if err != nil {
		return nil, err
	}
	rows, readErr := readWorksheetStream(source, sharedStrings, rowLimit)
	if readErr != nil {
		readErr = xerrors.Wrapf(readErr, "decode XLSX worksheet %q", sheetName)
	}
	return rows, errors.Join(readErr, source.Close())
}

func readWorksheetStream(source io.Reader, sharedStrings []string, rowLimit uint) ([][]string, error) {
	rows, err := parseWorksheet(source, sharedStrings, rowLimit)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(io.Discard, source); err != nil {
		return nil, err
	}
	return rows, nil
}
