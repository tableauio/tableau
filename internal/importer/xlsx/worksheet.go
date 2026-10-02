package xlsx

import (
	"archive/zip"
	"errors"
	"io"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

// readWorksheetRows buffers complete sheets for allocation-light decoding and
// streams bounded reads. Both paths verify the ZIP entry's checksum.
func readWorksheetRows(entry *zip.File, sheetName string, sharedStrings []string, rowLimit uint) ([][]string, error) {
	if rowLimit == 0 {
		data, err := readEntry(entry)
		if err != nil {
			return nil, err
		}
		rows, err := parseFullWorksheet(data, sharedStrings)
		if err != nil {
			return nil, xerrors.Wrapf(err, "decode XLSX worksheet %q", sheetName)
		}
		return rows, nil
	}
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
