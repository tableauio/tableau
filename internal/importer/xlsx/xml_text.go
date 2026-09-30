package xlsx

import (
	"bytes"
	"html"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

func hasUnsupportedXML(data []byte) bool {
	return bytes.Contains(data, []byte("<![CDATA[")) ||
		bytes.Contains(data, []byte("<!DOCTYPE"))
}

func decodeXMLText(value []byte) (string, error) {
	if !utf8.Valid(value) || bytes.Contains(value, []byte("]]>")) {
		return "", xerrors.New("invalid UTF-8 in XML text")
	}
	for remaining := value; len(remaining) > 0; {
		char, size := utf8.DecodeRune(remaining)
		if !validXMLRune(char) {
			return "", xerrors.Newf("invalid XML character %U", char)
		}
		remaining = remaining[size:]
	}
	if !bytes.ContainsAny(value, "\r&<") {
		return string(value), nil
	}

	var decoded strings.Builder
	decoded.Grow(len(value))
	segmentStart := 0
	for offset := 0; offset < len(value); {
		switch value[offset] {
		case '\r':
			decoded.Write(value[segmentStart:offset])
			decoded.WriteByte('\n')
			offset++
			if offset < len(value) && value[offset] == '\n' {
				offset++
			}
			segmentStart = offset
		case '&':
			decoded.Write(value[segmentStart:offset])
			end := bytes.IndexByte(value[offset+1:], ';')
			if end < 0 {
				return "", xerrors.New("unterminated entity in XML text")
			}
			end += offset + 1
			text, err := decodeXMLEntity(value[offset+1 : end])
			if err != nil {
				return "", err
			}
			decoded.WriteString(text)
			offset = end + 1
			segmentStart = offset
		case '<':
			decoded.Write(value[segmentStart:offset])
			var end int
			switch {
			case bytes.HasPrefix(value[offset:], []byte("<!--")):
				end = bytes.Index(value[offset+4:], []byte("-->"))
				if end >= 0 {
					end += offset + 7
				}
			case bytes.HasPrefix(value[offset:], []byte("<?")):
				end = bytes.Index(value[offset+2:], []byte("?>"))
				if end >= 0 {
					end += offset + 4
				}
			}
			if end <= offset {
				return "", xerrors.New("unexpected XML markup in text")
			}
			offset = end
			segmentStart = offset
		default:
			offset++
		}
	}
	if segmentStart == 0 {
		return string(value), nil
	}
	decoded.Write(value[segmentStart:])
	return decoded.String(), nil
}

func decodeXMLEntity(entity []byte) (string, error) {
	switch {
	case bytes.Equal(entity, []byte("amp")):
		return "&", nil
	case bytes.Equal(entity, []byte("lt")):
		return "<", nil
	case bytes.Equal(entity, []byte("gt")):
		return ">", nil
	case bytes.Equal(entity, []byte("apos")):
		return "'", nil
	case bytes.Equal(entity, []byte("quot")):
		return `"`, nil
	}
	if len(entity) == 0 || entity[0] != '#' {
		named := "&" + string(entity) + ";"
		decoded := html.UnescapeString(named)
		if decoded == named {
			return "", xerrors.Newf("invalid XML entity &%s;", entity)
		}
		return decoded, nil
	}

	base := 10
	number := entity[1:]
	if len(number) > 1 && number[0] == 'x' {
		base = 16
		number = number[1:]
	}
	value, err := strconv.ParseUint(string(number), base, 32)
	if err != nil || !validXMLRune(rune(value)) {
		return "", xerrors.Newf("invalid XML character reference &%s;", entity)
	}
	return string(rune(value)), nil
}

func validXMLRune(value rune) bool {
	return value == '\t' || value == '\n' || value == '\r' ||
		(value >= 0x20 && value <= 0xD7FF) ||
		(value >= 0xE000 && value <= 0xFFFD) ||
		(value >= 0x10000 && value <= 0x10FFFF)
}
