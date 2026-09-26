package xlsx

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"unicode/utf16"
)

const (
	maxRows    = 1_048_576
	maxColumns = 16_384
)

type tag struct {
	local       []byte
	attrs       []byte
	start       int
	end         int
	closing     bool
	selfClosing bool
}

func nextTag(data []byte, offset int) (tag, int, bool, error) {
	for offset < len(data) {
		relative := bytes.IndexByte(data[offset:], '<')
		if relative < 0 {
			return tag{}, len(data), false, nil
		}
		start := offset + relative
		if bytes.HasPrefix(data[start:], []byte("<!--")) {
			end := bytes.Index(data[start+4:], []byte("-->"))
			if end < 0 {
				return tag{}, len(data), false, fmt.Errorf("unterminated XML comment at byte %d", start)
			}
			offset = start + 4 + end + 3
			continue
		}
		if bytes.HasPrefix(data[start:], []byte("<?")) {
			end := bytes.Index(data[start+2:], []byte("?>"))
			if end < 0 {
				return tag{}, len(data), false, fmt.Errorf("unterminated XML processing instruction at byte %d", start)
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
				return tag{}, len(data), false, fmt.Errorf("unterminated XML declaration at byte %d", start)
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
			return tag{}, len(data), false, fmt.Errorf("unterminated XML tag at byte %d", start)
		}
		last := end - 1
		for last >= pos && isXMLSpace(data[last]) {
			last--
		}
		selfClosing := last >= pos && data[last] == '/'
		return tag{
			local:       local,
			attrs:       data[pos:end],
			start:       start,
			end:         end,
			closing:     closing,
			selfClosing: selfClosing,
		}, end + 1, true, nil
	}
	return tag{}, len(data), false, nil
}

type cell struct {
	kind       byte
	column     int
	value      []byte
	formula    bool
	inlineText strings.Builder
}

func parseRows(data []byte, sharedStrings []string) ([][]string, error) {
	rows := make([][]string, 0)
	var row []string
	var current cell
	var offset, rowNumber, lastRow, cellColumn, captureStart, phoneticDepth int
	var inRow, inCell bool
	var capture byte

	for {
		tag, next, ok, err := nextTag(data, offset)
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		offset = next
		if tag.closing {
			switch {
			case capture == 'v' && bytes.Equal(tag.local, []byte("v")):
				current.value = data[captureStart:tag.start]
				capture = 0
			case capture == 't' && bytes.Equal(tag.local, []byte("t")):
				current.inlineText.WriteString(decodeText(data[captureStart:tag.start]))
				capture = 0
			}
			switch {
			case bytes.Equal(tag.local, []byte("rPh")) && phoneticDepth > 0:
				phoneticDepth--
			case bytes.Equal(tag.local, []byte("c")) && inCell:
				var value string
				switch current.kind {
				case 's':
					index, err := parseInt(bytes.TrimSpace(current.value))
					if err == nil && index >= 0 && index < len(sharedStrings) {
						value = sharedStrings[index]
					} else {
						value = decodeText(current.value)
					}
				case 'i':
					value = decodeEscapes(current.inlineText.String())
				default:
					value = decodeText(current.value)
				}
				if value != "" || current.formula {
					if current.column <= 0 {
						current.column = cellColumn + 1
					}
					if current.column > len(row) {
						row = append(row, make([]string, current.column-len(row))...)
					}
					row[current.column-1] = value
				}
				cellColumn = current.column
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
			if value, ok := attribute(tag.attrs, 'r'); ok {
				parsed, err := parseInt(value)
				if err != nil || parsed < 1 || parsed > maxRows {
					return nil, fmt.Errorf("invalid XLSX row number %q", value)
				}
				rowNumber = parsed
			} else if rowNumber > maxRows {
				return nil, fmt.Errorf("XLSX row number exceeds %d", maxRows)
			}
			row = nil
			cellColumn = 0
			inRow = !tag.selfClosing
		case inRow && bytes.Equal(tag.local, []byte("c")):
			current = cell{column: cellColumn + 1}
			if reference, ok := attribute(tag.attrs, 'r'); ok {
				column := parseColumn(reference)
				if column < 1 || column > maxColumns {
					return nil, fmt.Errorf("invalid XLSX cell reference %q", reference)
				}
				current.column = column
			} else if current.column > maxColumns {
				return nil, fmt.Errorf("XLSX column number exceeds %d", maxColumns)
			}
			if kind, ok := attribute(tag.attrs, 't'); ok {
				switch {
				case bytes.Equal(kind, []byte("s")):
					current.kind = 's'
				case bytes.Equal(kind, []byte("inlineStr")):
					current.kind = 'i'
				}
			}
			inCell = !tag.selfClosing
			if tag.selfClosing {
				cellColumn = current.column
			}
		case inCell && bytes.Equal(tag.local, []byte("f")):
			current.formula = true
		case inCell && bytes.Equal(tag.local, []byte("rPh")):
			phoneticDepth++
		case inCell && bytes.Equal(tag.local, []byte("v")) && !tag.selfClosing:
			capture = 'v'
			captureStart = tag.end + 1
		case inCell && current.kind == 'i' && phoneticDepth == 0 && bytes.Equal(tag.local, []byte("t")) && !tag.selfClosing:
			capture = 't'
			captureStart = tag.end + 1
		}
	}
	if inRow || inCell || capture != 0 {
		return nil, errors.New("unterminated worksheet row or cell")
	}
	return rows, nil
}

func attribute(attrs []byte, name byte) ([]byte, bool) {
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
			if bytes.Equal(key[:colon], []byte("xmlns")) {
				key = nil
			} else {
				key = key[colon+1:]
			}
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

func decodeText(value []byte) string {
	text := string(value)
	if strings.Contains(text, "&") {
		text = html.UnescapeString(text)
	}
	return decodeEscapes(text)
}

func decodeEscapes(value string) string {
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
		code, end, ok := parseEscape(value, start)
		if ok {
			decoded.WriteString(value[cursor:start])
			decodedRune := rune(code)
			if code >= 0xD800 && code <= 0xDBFF {
				if low, lowEnd, ok := parseEscape(value, end); ok && low >= 0xDC00 && low <= 0xDFFF {
					decodedRune = utf16.DecodeRune(decodedRune, rune(low))
					end = lowEnd
				}
			}
			decoded.WriteRune(decodedRune)
			cursor = end
			continue
		}
		decoded.WriteString(value[cursor : start+2])
		cursor = start + 2
	}
	decoded.WriteString(value[cursor:])
	return decoded.String()
}

func parseEscape(value string, start int) (uint16, int, bool) {
	const escapeLength = len("_x0000_")
	end := start + escapeLength
	if start < 0 || end > len(value) || !strings.HasPrefix(value[start:], "_x") || value[end-1] != '_' {
		return 0, start, false
	}
	code, err := strconv.ParseUint(value[start+2:end-1], 16, 16)
	return uint16(code), end, err == nil
}

func isXMLSpace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\r' || char == '\n'
}
