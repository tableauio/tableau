package xlsx

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
)

const initialWorksheetPrefixSize = 64 << 10

// readWorksheetRowsN parses a growing prefix until the requested rows are
// complete, then streams the remaining ZIP data to verify its checksum.
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
		if err := validateWorksheetXML(data, sheetName); err != nil {
			return nil, err
		}
		rows, parseErr := parseRowsN(data, sharedStrings, rowLimit)
		if parseErr != nil && !errors.Is(parseErr, errIncompleteXML) {
			return nil, parseErr
		}
		if len(data) == uncompressedSize {
			if err := ensureEntryExhausted(source, entry.Name); err != nil {
				return nil, err
			}
			return rows, parseErr
		}

		if parseErr == nil && uint(len(rows)) >= rowLimit {
			if err := verifyWorksheetTail(source, entry, data, sheetName); err != nil {
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

// verifyWorksheetTail drains the ZIP entry so checksum failures cannot be
// hidden by a successful prefix parse. It checks for unsupported constructs
// across read boundaries without retaining the rest of the worksheet.
func verifyWorksheetTail(source io.Reader, entry *zip.File, prefix []byte, sheetName string) error {
	const overlap = len("<![CDATA[") - 1
	validator := &worksheetTailValidator{
		sheetName: sheetName,
		tail:      append([]byte(nil), prefix[max(0, len(prefix)-overlap):]...),
	}
	count, err := io.Copy(validator, source)
	if err != nil {
		return err
	}
	if count != int64(entry.UncompressedSize64)-int64(len(prefix)) {
		return fmt.Errorf("XLSX entry %q size differs from its ZIP header", entry.Name)
	}
	return nil
}

type worksheetTailValidator struct {
	sheetName string
	tail      []byte
}

func (v *worksheetTailValidator) Write(data []byte) (int, error) {
	window := make([]byte, 0, len(v.tail)+len(data))
	window = append(window, v.tail...)
	window = append(window, data...)
	if err := validateWorksheetXML(window, v.sheetName); err != nil {
		return 0, err
	}
	const overlap = len("<![CDATA[") - 1
	v.tail = append(v.tail[:0], window[max(0, len(window)-overlap):]...)
	return len(data), nil
}

func validateWorksheetXML(data []byte, sheetName string) error {
	if hasUnsupportedXML(data) {
		return fmt.Errorf("%w: XML construct in worksheet %q", ErrUnsupported, sheetName)
	}
	return nil
}
