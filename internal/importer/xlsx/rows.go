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

// worksheetRows owns row placement for both worksheet decoders. Repeated row
// numbers and cell coordinates follow Excelize's raw-row placement.
type worksheetRows struct {
	rows          [][]string
	row           []string
	cell          worksheetCell
	shared        []string
	sharedValue   func(int) (string, error)
	rowNumber     int
	cellColumn    int
	rowLimit      int
	inRow         bool
	inCell        bool
	phoneticDepth int
}

func newWorksheetRows(shared []string, rowLimit uint) *worksheetRows {
	p := &worksheetRows{
		shared:   shared,
		rowLimit: int(min(rowLimit, uint(maxRows))),
	}
	if rowLimit == 0 {
		p.rows = make([][]string, 0)
	}
	return p
}

// startRow reports whether a bounded read has reached an unrequested row.
func (p *worksheetRows) startRow(reference string, present bool) (bool, error) {
	p.rowNumber++
	if present {
		number, err := parseInt([]byte(reference))
		if err != nil || number < 1 || number > maxRows {
			return false, xerrors.Newf("invalid XLSX row number %q", reference)
		}
		p.rowNumber = number
	} else if p.rowNumber > maxRows {
		return false, xerrors.Newf("XLSX row number exceeds %d", maxRows)
	}
	if p.rowLimit > 0 && p.rowNumber > p.rowLimit {
		for len(p.rows) < p.rowLimit {
			p.rows = append(p.rows, nil)
		}
		return true, nil
	}
	p.row = nil
	if p.rowNumber <= len(p.rows) {
		p.row = p.rows[p.rowNumber-1]
	}
	p.cellColumn = 0
	p.inRow = true
	return false, nil
}

func (p *worksheetRows) startCell(reference, kind string, present bool) error {
	p.cell = worksheetCell{column: p.cellColumn + 1, kind: kind}
	p.phoneticDepth = 0
	if present {
		p.cell.column = parseColumn([]byte(reference))
		if p.cell.column < 1 || p.cell.column > maxColumns {
			return xerrors.Newf("invalid XLSX cell reference %q", reference)
		}
	} else if p.cell.column > maxColumns {
		return xerrors.Newf("XLSX column number exceeds %d", maxColumns)
	}
	p.inCell = true
	return nil
}

func (p *worksheetRows) endCell() error {
	var value string
	var err error
	if p.sharedValue != nil && p.cell.kind == "s" && p.cell.value != "" {
		var index int
		index, err = p.cell.sharedIndex()
		if err == nil {
			value, err = p.sharedValue(index)
		}
	} else {
		value, err = p.cell.text(p.shared)
	}
	if err != nil {
		return err
	}
	if value != "" || p.cell.formula {
		if p.cell.column > len(p.row)+1 {
			oldLength := len(p.row)
			p.row = slices.Grow(p.row, p.cell.column-oldLength)
			p.row = p.row[:p.cell.column-1]
			clear(p.row[oldLength:])
		}
		p.row = append(p.row, value)
	}
	p.cellColumn = p.cell.column
	p.inCell = false
	return nil
}

func (p *worksheetRows) endRow() {
	if p.rowLimit > 0 || len(p.row) > 0 {
		for len(p.rows) < p.rowNumber {
			p.rows = append(p.rows, nil)
		}
		p.rows[p.rowNumber-1] = p.row
	}
	p.inRow = false
}

// parseWorksheet collects requested rows using the strict XML decoder. It is
// also the compatibility path for XML outside the full-sheet fast path.
func parseWorksheet(source io.Reader, sharedStrings []string, rowLimit uint) ([][]string, error) {
	p := newWorksheetRows(sharedStrings, rowLimit)
	return parseWorksheetWithRows(source, p)
}

func parseWorksheetWithRows(source io.Reader, p *worksheetRows) ([][]string, error) {
	decoder := newXMLDecoder(source)
	for {
		token, err := nextXMLToken(decoder)
		if err == io.EOF {
			return p.rows, nil
		}
		if err != nil {
			return nil, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			switch token.Name.Local {
			case "row":
				reference, present := xmlAttribute(token, "r")
				stop, err := p.startRow(reference, present)
				if err != nil {
					return nil, err
				}
				if stop {
					return p.rows, nil
				}
			case "c":
				if p.inRow {
					reference, present := xmlAttribute(token, "r")
					kind, _ := xmlAttribute(token, "t")
					if err := p.startCell(reference, kind, present); err != nil {
						return nil, err
					}
				}
			case "f":
				if p.inCell {
					p.cell.formula = true
				}
			case "rPh":
				if p.inCell {
					p.phoneticDepth++
				}
			case "v":
				if p.inCell {
					p.cell.value, err = readXMLText(decoder)
					if err != nil {
						return nil, err
					}
				}
			case "t":
				if p.inCell && p.cell.kind == "inlineStr" && p.phoneticDepth == 0 {
					text, err := readXMLText(decoder)
					if err != nil {
						return nil, err
					}
					p.cell.inlineText.WriteString(text)
				}
			}
		case xml.EndElement:
			switch token.Name.Local {
			case "rPh":
				if p.phoneticDepth > 0 {
					p.phoneticDepth--
				}
			case "c":
				if p.inCell {
					if err := p.endCell(); err != nil {
						return nil, err
					}
				}
			case "row":
				p.endRow()
			case "sheetData", "worksheet":
				if p.rowLimit > 0 {
					return p.rows, nil
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
		index, err := cell.sharedIndex()
		if err != nil {
			return "", err
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

func (cell *worksheetCell) sharedIndex() (int, error) {
	index, err := parseInt([]byte(strings.TrimSpace(cell.value)))
	if err != nil {
		return 0, xerrors.Wrapf(err, "invalid shared string index %q", cell.value)
	}
	return index, nil
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
