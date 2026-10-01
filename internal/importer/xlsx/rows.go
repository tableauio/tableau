package xlsx

import (
	"encoding/xml"
	"io"
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
	kind       string
	column     int
	value      string
	formula    bool
	inlineText strings.Builder
}

// parseWorksheet collects only requested rows from XML events. Repeated row
// numbers and cell coordinates follow Excelize's raw-row placement.
func parseWorksheet(source io.Reader, sharedStrings []string, rowLimit uint) ([][]string, error) {
	decoder := newXMLDecoder(source)
	var rows [][]string
	if rowLimit == 0 {
		rows = make([][]string, 0)
	}
	var row []string
	var cell worksheetCell
	var rowNumber, cellColumn, phoneticDepth int
	var inRow, inCell bool
	maxReturnedRows := 0
	if rowLimit > 0 {
		maxReturnedRows = int(min(rowLimit, uint(maxRows)))
	}

	for {
		token, err := nextXMLToken(decoder)
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			switch token.Name.Local {
			case "row":
				rowNumber++
				if value, ok := xmlAttribute(token, "r"); ok {
					parsed, err := parseInt([]byte(value))
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
				inRow = true
			case "c":
				if !inRow {
					continue
				}
				cell = worksheetCell{column: cellColumn + 1}
				phoneticDepth = 0
				if reference, ok := xmlAttribute(token, "r"); ok {
					cell.column = parseColumn([]byte(reference))
					if cell.column < 1 || cell.column > maxColumns {
						return nil, xerrors.Newf("invalid XLSX cell reference %q", reference)
					}
				} else if cell.column > maxColumns {
					return nil, xerrors.Newf("XLSX column number exceeds %d", maxColumns)
				}
				cell.kind, _ = xmlAttribute(token, "t")
				inCell = true
			case "f":
				if inCell {
					cell.formula = true
				}
			case "rPh":
				if inCell {
					phoneticDepth++
				}
			case "v":
				if inCell {
					cell.value, err = readXMLText(decoder)
					if err != nil {
						return nil, err
					}
				}
			case "t":
				if inCell && cell.kind == "inlineStr" && phoneticDepth == 0 {
					text, err := readXMLText(decoder)
					if err != nil {
						return nil, err
					}
					cell.inlineText.WriteString(text)
				}
			}
		case xml.EndElement:
			switch token.Name.Local {
			case "rPh":
				if phoneticDepth > 0 {
					phoneticDepth--
				}
			case "c":
				if !inCell {
					continue
				}
				value, err := cell.text(sharedStrings)
				if err != nil {
					return nil, err
				}
				if value != "" || cell.formula {
					if cell.column > len(row)+1 {
						oldLength := len(row)
						row = slices.Grow(row, cell.column-oldLength)
						row = row[:cell.column-1]
						clear(row[oldLength:])
					}
					row = append(row, value)
				}
				cellColumn = cell.column
				inCell = false
			case "row":
				if maxReturnedRows > 0 || len(row) > 0 {
					for len(rows) < rowNumber {
						rows = append(rows, nil)
					}
					rows[rowNumber-1] = row
				}
				inRow = false
			case "sheetData", "worksheet":
				if rowLimit > 0 {
					return rows, nil
				}
			}
		}
	}
}

func (cell *worksheetCell) text(sharedStrings []string) (string, error) {
	switch cell.kind {
	case "s":
		if cell.value == "" {
			return "", nil
		}
		index, err := parseInt([]byte(strings.TrimSpace(cell.value)))
		if err != nil {
			return "", xerrors.Wrapf(err, "invalid shared string index %q", cell.value)
		}
		if index < 0 || index >= len(sharedStrings) {
			return "", xerrors.Newf("shared string index %d out of range", index)
		}
		return sharedStrings[index], nil
	case "inlineStr":
		return decodeEscapes(cell.inlineText.String()), nil
	case "str":
		return cell.value, nil
	default:
		return decodeEscapes(cell.value), nil
	}
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
