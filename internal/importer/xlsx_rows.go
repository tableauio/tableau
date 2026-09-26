package importer

import (
	"bytes"
	"html"
	"strconv"
	"strings"
)

type xlsxTag struct {
	local       []byte
	attrs       []byte
	start       int
	end         int
	closing     bool
	selfClosing bool
}

func nextXLSXTag(data []byte, offset int) (xlsxTag, int, bool) {
	for offset < len(data) {
		relative := bytes.IndexByte(data[offset:], '<')
		if relative < 0 {
			return xlsxTag{}, len(data), false
		}
		start := offset + relative
		if bytes.HasPrefix(data[start:], []byte("<!--")) {
			end := bytes.Index(data[start+4:], []byte("-->"))
			if end < 0 {
				return xlsxTag{}, len(data), false
			}
			offset = start + 4 + end + 3
			continue
		}
		if bytes.HasPrefix(data[start:], []byte("<?")) {
			end := bytes.Index(data[start+2:], []byte("?>"))
			if end < 0 {
				return xlsxTag{}, len(data), false
			}
			offset = start + 2 + end + 2
			continue
		}
		pos := start + 1
		closing := false
		if pos < len(data) && data[pos] == '/' {
			closing = true
			pos++
		}
		for pos < len(data) && isXMLSpace(data[pos]) {
			pos++
		}
		nameStart := pos
		for pos < len(data) && !isXMLSpace(data[pos]) && data[pos] != '/' && data[pos] != '>' {
			pos++
		}
		if nameStart == pos || data[nameStart] == '!' {
			end := bytes.IndexByte(data[pos:], '>')
			if end < 0 {
				return xlsxTag{}, len(data), false
			}
			offset = pos + end + 1
			continue
		}
		name := data[nameStart:pos]
		local := name
		if colon := bytes.LastIndexByte(name, ':'); colon >= 0 {
			local = name[colon+1:]
		}
		quote := byte(0)
		end := pos
		for end < len(data) {
			char := data[end]
			if quote != 0 {
				if char == quote {
					quote = 0
				}
			} else if char == '\'' || char == '"' {
				quote = char
			} else if char == '>' {
				break
			}
			end++
		}
		if end >= len(data) {
			return xlsxTag{}, len(data), false
		}
		last := end - 1
		for last >= pos && isXMLSpace(data[last]) {
			last--
		}
		selfClosing := last >= pos && data[last] == '/'
		return xlsxTag{
			local:       local,
			attrs:       data[pos:end],
			start:       start,
			end:         end,
			closing:     closing,
			selfClosing: selfClosing,
		}, end + 1, true
	}
	return xlsxTag{}, len(data), false
}

type xlsxCell struct {
	kind       byte
	column     int
	value      []byte
	formula    bool
	inlineText strings.Builder
}

func parseXLSXRows(data []byte, sharedStrings []string) ([][]string, error) {
	rows := make([][]string, 0)
	var row []string
	var cell xlsxCell
	var offset, rowNumber, lastRow, cellColumn, captureStart, phoneticDepth int
	var inRow, inCell bool
	var capture byte

	for {
		tag, next, ok := nextXLSXTag(data, offset)
		if !ok {
			break
		}
		offset = next
		if tag.closing {
			switch {
			case capture == 'v' && bytes.Equal(tag.local, []byte("v")):
				cell.value = data[captureStart:tag.start]
				capture = 0
			case capture == 't' && bytes.Equal(tag.local, []byte("t")):
				cell.inlineText.WriteString(decodeXMLText(data[captureStart:tag.start]))
				capture = 0
			}
			switch {
			case bytes.Equal(tag.local, []byte("rPh")) && phoneticDepth > 0:
				phoneticDepth--
			case bytes.Equal(tag.local, []byte("c")) && inCell:
				var value string
				switch cell.kind {
				case 's':
					index, err := parseXLSXInt(bytes.TrimSpace(cell.value))
					if err == nil && index >= 0 && index < len(sharedStrings) {
						value = sharedStrings[index]
					} else {
						value = decodeXMLText(cell.value)
					}
				case 'i':
					value = decodeExcelEscapes(cell.inlineText.String())
				default:
					value = decodeXMLText(cell.value)
				}
				if value != "" || cell.formula {
					if cell.column <= 0 {
						cell.column = cellColumn + 1
					}
					if cell.column > len(row) {
						row = append(row, make([]string, cell.column-len(row))...)
					}
					row[cell.column-1] = value
				}
				cellColumn = cell.column
				inCell = false
			case bytes.Equal(tag.local, []byte("row")) && inRow:
				if len(row) > 0 {
					if emptyRows := rowNumber - lastRow - 1; emptyRows > 0 {
						rows = append(rows, make([][]string, emptyRows)...)
					}
					rows = append(rows, row)
					lastRow = rowNumber
				}
				inRow = false
			}
			continue
		}

		switch {
		case bytes.Equal(tag.local, []byte("row")):
			rowNumber++
			if value, ok := xlsxAttribute(tag.attrs, 'r'); ok {
				if parsed, err := parseXLSXInt(value); err == nil && parsed > 0 {
					rowNumber = parsed
				}
			}
			row = nil
			cellColumn = 0
			inRow = true
		case inRow && bytes.Equal(tag.local, []byte("c")):
			cell = xlsxCell{column: cellColumn + 1}
			if reference, ok := xlsxAttribute(tag.attrs, 'r'); ok {
				if column := parseXLSXColumn(reference); column > 0 {
					cell.column = column
				}
			}
			if kind, ok := xlsxAttribute(tag.attrs, 't'); ok {
				switch {
				case bytes.Equal(kind, []byte("s")):
					cell.kind = 's'
				case bytes.Equal(kind, []byte("inlineStr")):
					cell.kind = 'i'
				}
			}
			inCell = !tag.selfClosing
			if tag.selfClosing {
				cellColumn = cell.column
			}
		case inCell && bytes.Equal(tag.local, []byte("f")):
			cell.formula = true
		case inCell && bytes.Equal(tag.local, []byte("rPh")):
			phoneticDepth++
		case inCell && bytes.Equal(tag.local, []byte("v")) && !tag.selfClosing:
			capture = 'v'
			captureStart = tag.end + 1
		case inCell && cell.kind == 'i' && phoneticDepth == 0 && bytes.Equal(tag.local, []byte("t")) && !tag.selfClosing:
			capture = 't'
			captureStart = tag.end + 1
		}
	}
	return rows, nil
}

func xlsxAttribute(attrs []byte, name byte) ([]byte, bool) {
	for offset := 0; offset < len(attrs); {
		for offset < len(attrs) && (isXMLSpace(attrs[offset]) || attrs[offset] == '/') {
			offset++
		}
		start := offset
		for offset < len(attrs) && !isXMLSpace(attrs[offset]) && attrs[offset] != '=' && attrs[offset] != '/' {
			offset++
		}
		if start == offset {
			break
		}
		key := attrs[start:offset]
		if colon := bytes.LastIndexByte(key, ':'); colon >= 0 {
			key = key[colon+1:]
		}
		for offset < len(attrs) && isXMLSpace(attrs[offset]) {
			offset++
		}
		if offset >= len(attrs) || attrs[offset] != '=' {
			continue
		}
		offset++
		for offset < len(attrs) && isXMLSpace(attrs[offset]) {
			offset++
		}
		if offset >= len(attrs) || (attrs[offset] != '\'' && attrs[offset] != '"') {
			continue
		}
		quote := attrs[offset]
		offset++
		valueStart := offset
		for offset < len(attrs) && attrs[offset] != quote {
			offset++
		}
		value := attrs[valueStart:offset]
		if offset < len(attrs) {
			offset++
		}
		if len(key) == 1 && key[0] == name {
			return value, true
		}
	}
	return nil, false
}

func parseXLSXColumn(reference []byte) int {
	column := 0
	for _, char := range reference {
		switch {
		case char == '$':
			continue
		case char >= 'A' && char <= 'Z':
			column = column*26 + int(char-'A'+1)
		case char >= 'a' && char <= 'z':
			column = column*26 + int(char-'a'+1)
		default:
			return column
		}
	}
	return column
}

func parseXLSXInt(value []byte) (int, error) {
	if len(value) == 0 {
		return 0, strconv.ErrSyntax
	}
	parsed := 0
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, strconv.ErrSyntax
		}
		parsed = parsed*10 + int(char-'0')
	}
	return parsed, nil
}

func decodeXMLText(value []byte) string {
	text := string(value)
	if strings.Contains(text, "&") {
		text = html.UnescapeString(text)
	}
	return decodeExcelEscapes(text)
}

func decodeExcelEscapes(value string) string {
	if !strings.Contains(value, "_x") {
		return value
	}
	var decoded strings.Builder
	cursor := 0
	for cursor < len(value) {
		relative := strings.Index(value[cursor:], "_x")
		if relative < 0 {
			break
		}
		start := cursor + relative
		end := start + 7
		if end <= len(value) && value[end-1] == '_' {
			if code, err := strconv.ParseUint(value[start+2:end-1], 16, 16); err == nil {
				decoded.WriteString(value[cursor:start])
				decoded.WriteRune(rune(code))
				cursor = end
				continue
			}
		}
		decoded.WriteString(value[cursor : start+2])
		cursor = start + 2
	}
	decoded.WriteString(value[cursor:])
	return decoded.String()
}

func isXMLSpace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\r' || char == '\n'
}
