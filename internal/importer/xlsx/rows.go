package xlsx

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

const (
	maxRows    = 1_048_576
	maxColumns = 16_384
)

type worksheetCell struct {
	kind       byte
	column     int
	value      []byte
	formula    bool
	inlineText strings.Builder
}

func parseRows(data []byte, sharedStrings []string) ([][]string, error) {
	return parseRowsN(data, sharedStrings, 0)
}

// parseRowsN parses the first rowLimit logical worksheet rows. A zero limit
// keeps the GetRows-compatible behavior of omitting trailing empty rows. A
// positive limit mirrors the Excelize row iterator, including empty rows before
// the last worksheet row.
func parseRowsN(data []byte, sharedStrings []string, rowLimit uint) ([][]string, error) {
	var rows [][]string
	if rowLimit == 0 {
		rows = make([][]string, 0)
	}
	var row []string
	var current worksheetCell
	var offset, rowNumber, cellColumn, captureStart, phoneticDepth int
	var inRow, inCell bool
	var sheetDataClosed bool
	var capture byte
	maxReturnedRows := 0
	if rowLimit > 0 {
		maxReturnedRows = maxRows
		if rowLimit < uint(maxRows) {
			maxReturnedRows = int(rowLimit)
		}
	}

	for {
		element, nextOffset, ok, err := nextXMLTag(data, offset)
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		offset = nextOffset
		if element.closing {
			switch {
			case capture == 'v' && bytes.Equal(element.local, []byte("v")):
				current.value = data[captureStart:element.start]
				capture = 0
			case capture == 't' && bytes.Equal(element.local, []byte("t")):
				text, err := decodeXMLText(data[captureStart:element.start])
				if err != nil {
					return nil, err
				}
				current.inlineText.WriteString(text)
				capture = 0
			}
			switch {
			case bytes.Equal(element.local, []byte("rPh")) && phoneticDepth > 0:
				phoneticDepth--
			case bytes.Equal(element.local, []byte("c")) && inCell:
				var value string
				switch current.kind {
				case 's':
					index, err := parseInt(bytes.TrimSpace(current.value))
					if err != nil {
						return nil, xerrors.Wrapf(err, "invalid shared string index %q", current.value)
					}
					if index < 0 || index >= len(sharedStrings) {
						return nil, xerrors.Newf("shared string index %d out of range", index)
					}
					value = sharedStrings[index]
				case 'i':
					value = decodeEscapes(current.inlineText.String())
				case 'r':
					value, err = decodeXMLText(current.value)
					if err != nil {
						return nil, err
					}
				default:
					value, err = decodeText(current.value)
					if err != nil {
						return nil, err
					}
				}
				if value != "" || current.formula {
					if current.column <= 0 {
						current.column = cellColumn + 1
					}
					if current.column > len(row)+1 {
						oldLength := len(row)
						row = slices.Grow(row, current.column-oldLength)
						row = row[:current.column-1]
						clear(row[oldLength:])
					}
					row = append(row, value)
				}
				cellColumn = current.column
				phoneticDepth = 0
				inCell = false
			case bytes.Equal(element.local, []byte("row")) && inRow:
				if maxReturnedRows > 0 {
					for len(rows) < rowNumber && len(rows) < maxReturnedRows {
						rows = append(rows, nil)
					}
					if rowNumber <= maxReturnedRows {
						rows[rowNumber-1] = row
					}
				} else if len(row) > 0 {
					for len(rows) < rowNumber {
						rows = append(rows, nil)
					}
					rows[rowNumber-1] = row
				}
				inRow = false
			case bytes.Equal(element.local, []byte("sheetData")) ||
				bytes.Equal(element.local, []byte("worksheet")):
				sheetDataClosed = true
			}
			continue
		}

		switch {
		case element.selfClosing && (bytes.Equal(element.local, []byte("sheetData")) ||
			bytes.Equal(element.local, []byte("worksheet"))):
			sheetDataClosed = true
		case bytes.Equal(element.local, []byte("row")):
			rowNumber++
			value, ok, err := plainXMLAttribute(element.attributes, 'r')
			if err != nil {
				return nil, err
			}
			if ok {
				parsed, err := parseInt(value)
				if err != nil || parsed < 1 || parsed > maxRows {
					return nil, xerrors.Newf("invalid XLSX row number %q", value)
				}
				rowNumber = parsed
			} else if rowNumber > maxRows {
				return nil, xerrors.Newf("XLSX row number exceeds %d", maxRows)
			}
			if maxReturnedRows > 0 && rowNumber > maxReturnedRows {
				for len(rows) < maxReturnedRows {
					rows = append(rows, nil)
				}
				return rows, nil
			}
			row = nil
			if rowNumber <= len(rows) {
				row = rows[rowNumber-1]
			}
			cellColumn = 0
			inRow = !element.selfClosing
			if element.selfClosing && maxReturnedRows > 0 {
				for len(rows) < rowNumber && len(rows) < maxReturnedRows {
					rows = append(rows, nil)
				}
			}
		case inRow && bytes.Equal(element.local, []byte("c")):
			current = worksheetCell{column: cellColumn + 1}
			phoneticDepth = 0
			reference, ok, err := plainXMLAttribute(element.attributes, 'r')
			if err != nil {
				return nil, err
			}
			if ok {
				column := parseColumn(reference)
				if column < 1 || column > maxColumns {
					return nil, xerrors.Newf("invalid XLSX cell reference %q", reference)
				}
				current.column = column
			} else if current.column > maxColumns {
				return nil, xerrors.Newf("XLSX column number exceeds %d", maxColumns)
			}
			kind, ok, err := plainXMLAttribute(element.attributes, 't')
			if err != nil {
				return nil, err
			}
			if ok {
				switch {
				case bytes.Equal(kind, []byte("s")):
					current.kind = 's'
				case bytes.Equal(kind, []byte("inlineStr")):
					current.kind = 'i'
				case bytes.Equal(kind, []byte("str")):
					current.kind = 'r'
				}
			}
			inCell = !element.selfClosing
			if element.selfClosing {
				cellColumn = current.column
			}
		case inCell && bytes.Equal(element.local, []byte("f")):
			current.formula = true
		case inCell && bytes.Equal(element.local, []byte("rPh")) && !element.selfClosing:
			phoneticDepth++
		case inCell && bytes.Equal(element.local, []byte("v")) && !element.selfClosing:
			capture = 'v'
			captureStart = element.end + 1
		case inCell && current.kind == 'i' && phoneticDepth == 0 &&
			bytes.Equal(element.local, []byte("t")) && !element.selfClosing:
			capture = 't'
			captureStart = element.end + 1
		}
	}
	if inRow || inCell || capture != 0 {
		return nil, fmt.Errorf("%w: worksheet row or cell", errIncompleteXML)
	}
	if rowLimit > 0 && !sheetDataClosed {
		return nil, errIncompleteXML
	}
	return rows, nil
}

func parseColumn(reference []byte) int {
	column := 0
	maxInt := int(^uint(0) >> 1)
	for _, char := range reference {
		var digit int
		switch {
		case char == '$':
			continue
		case char >= 'A' && char <= 'Z':
			digit = int(char - 'A' + 1)
		case char >= 'a' && char <= 'z':
			digit = int(char - 'a' + 1)
		default:
			return column
		}
		if column > (maxInt-digit)/26 {
			return 0
		}
		column = column*26 + digit
	}
	return column
}

func parseInt(value []byte) (int, error) {
	if len(value) == 0 {
		return 0, strconv.ErrSyntax
	}
	parsed := 0
	maxInt := int(^uint(0) >> 1)
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, strconv.ErrSyntax
		}
		digit := int(char - '0')
		if parsed > (maxInt-digit)/10 {
			return 0, strconv.ErrRange
		}
		parsed = parsed*10 + digit
	}
	return parsed, nil
}

func decodeText(value []byte) (string, error) {
	text, err := decodeXMLText(value)
	if err != nil {
		return "", err
	}
	return decodeEscapes(text), nil
}
