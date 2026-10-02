package xlsx

import (
	"strconv"
	"strings"
	"unicode/utf16"
)

// decodeEscapes decodes SpreadsheetML _xNNNN_ UTF-16 escape sequences.
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
