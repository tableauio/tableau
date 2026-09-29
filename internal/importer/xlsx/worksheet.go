package xlsx

import (
	"archive/zip"
	"errors"
	"io"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

const initialWorksheetPrefixSize = 64 << 10

// readWorksheetRowsN inflates a geometrically growing worksheet prefix until
// the requested logical rows are complete.
func readWorksheetRowsN(entry *zip.File, sheetName string, sharedStrings []string, rowLimit uint) ([][]string, error) {
	if _, err := validatedEntrySize(entry); err != nil {
		return nil, err
	}
	source, err := entry.Open()
	if err != nil {
		return nil, err
	}
	rows, readErr := readWorksheetPrefix(source, entry, sheetName, sharedStrings, rowLimit)
	return rows, errors.Join(readErr, source.Close())
}

func readWorksheetPrefix(source io.Reader, entry *zip.File, sheetName string, sharedStrings []string, rowLimit uint) ([][]string, error) {
	uncompressedSize := int(entry.UncompressedSize64)
	prefixSize := min(uncompressedSize, initialWorksheetPrefixSize)
	data := make([]byte, prefixSize)
	if _, err := io.ReadFull(source, data); err != nil {
		return nil, err
	}

	for {
		rows, parseErr := parseRowsN(data, sharedStrings, rowLimit)
		if len(data) == uncompressedSize {
			if err := ensureEntryExhausted(source, entry.Name); err != nil {
				return nil, err
			}
			if err := validateWorksheetXML(data, sheetName); err != nil {
				return nil, err
			}
			return rows, parseErr
		}

		if parseErr == nil && uint(len(rows)) >= rowLimit {
			if err := validateWorksheetXML(data, sheetName); err != nil {
				return nil, err
			}
			return rows, nil
		}

		nextSize := min(uncompressedSize, max(initialWorksheetPrefixSize, len(data)*2))
		nextData := make([]byte, nextSize)
		copy(nextData, data)
		if _, err := io.ReadFull(source, nextData[len(data):]); err != nil {
			return nil, err
		}
		data = nextData
	}
}

func validateWorksheetXML(data []byte, sheetName string) error {
	if hasUnsupportedXML(data) {
		return xerrors.Newf("unsupported XML construct in worksheet %q", sheetName)
	}
	return nil
}
