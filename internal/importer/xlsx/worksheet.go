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
	return readWorksheetWithRows(entry, sheetName, newWorksheetRows(sharedStrings, rowLimit))
}

func readWorksheetWithRows(entry *zip.File, sheetName string, p *worksheetRows) ([][]string, error) {
	if p.rowLimit == 0 {
		data, err := readEntry(entry)
		if err != nil {
			return nil, err
		}
		rows, err := parseFullWorksheetWithRows(data, p)
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
	rows, readErr := readWorksheetStreamWithRows(source, p)
	if readErr != nil {
		readErr = xerrors.Wrapf(readErr, "decode XLSX worksheet %q", sheetName)
	}
	return rows, errors.Join(readErr, source.Close())
}

func readWorksheetStream(source io.Reader, sharedStrings []string, rowLimit uint) ([][]string, error) {
	return readWorksheetStreamWithRows(source, newWorksheetRows(sharedStrings, rowLimit))
}

func readWorksheetStreamWithRows(source io.Reader, p *worksheetRows) ([][]string, error) {
	rows, err := parseWorksheetWithRows(source, p)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(io.Discard, source); err != nil {
		return nil, err
	}
	return rows, nil
}
