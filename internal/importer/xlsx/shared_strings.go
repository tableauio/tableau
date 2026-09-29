package xlsx

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

// parseSharedStrings decodes the small OOXML subset used by sharedStrings.xml
// without building an XML object tree. Unsupported constructs return an error
// so callers can retry the workbook with the compatibility reader.
func parseSharedStrings(data []byte) ([]string, error) {
	if hasUnsupportedSharedStringXML(data) {
		return nil, xerrors.New("unsupported XML construct in shared strings")
	}

	var (
		shared       []string
		stack        [][]byte
		offset       int
		itemDepth    int
		runDepth     int
		captureStart int
		inItem       bool
		inRun        bool
		inText       bool
		directSeen   bool
		runTextSeen  bool
		hasRuns      bool
		directText   string
		runText      strings.Builder
		itemText     strings.Builder
		rootSeen     bool
		rootClosed   bool
	)

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
			if len(stack) == 0 || !bytes.Equal(stack[len(stack)-1], tag.local) {
				return nil, xerrors.Newf("mismatched shared strings closing tag %q", tag.local)
			}
			depth := len(stack)
			if depth == 1 {
				rootClosed = true
			}
			switch {
			case inText && bytes.Equal(tag.local, []byte("t")):
				text, err := decodeXMLText(data[captureStart:tag.start])
				if err != nil {
					return nil, err
				}
				if inRun {
					runText.WriteString(text)
				} else {
					directText = text
				}
				inText = false
			case inRun && depth == runDepth && bytes.Equal(tag.local, []byte("r")):
				itemText.WriteString(runText.String())
				runText.Reset()
				inRun = false
				runDepth = 0
				runTextSeen = false
			case inItem && depth == itemDepth && bytes.Equal(tag.local, []byte("si")):
				if directSeen && hasRuns {
					return nil, xerrors.New("shared string mixes direct text and rich-text runs")
				}
				value := itemText.String()
				if directSeen {
					value = directText
				}
				shared = append(shared, decodeEscapes(value))
				itemText.Reset()
				directText = ""
				directSeen = false
				hasRuns = false
				inItem = false
				itemDepth = 0
			}
			stack = stack[:len(stack)-1]
			continue
		}

		if inText {
			return nil, xerrors.New("nested XML element in shared string text")
		}

		parentDepth := len(stack)
		if parentDepth == 0 {
			if rootSeen {
				return nil, xerrors.New("multiple shared strings root elements")
			}
			rootSeen = true
			rootClosed = tag.selfClosing
		}
		switch {
		case !inItem && parentDepth == 1 && bytes.Equal(tag.local, []byte("si")):
			if tag.selfClosing {
				shared = append(shared, "")
				continue
			}
			inItem = true
			itemDepth = parentDepth + 1
			directSeen = false
			hasRuns = false
			directText = ""
		case inItem && parentDepth == itemDepth && bytes.Equal(tag.local, []byte("r")):
			if tag.selfClosing {
				hasRuns = true
				continue
			}
			inRun = true
			runDepth = parentDepth + 1
			runTextSeen = false
			hasRuns = true
		case inItem && bytes.Equal(tag.local, []byte("t")) &&
			(parentDepth == itemDepth || (inRun && parentDepth == runDepth)):
			if inRun {
				if runTextSeen {
					return nil, xerrors.New("rich-text run has multiple text elements")
				}
				runTextSeen = true
			} else {
				if directSeen {
					return nil, xerrors.New("shared string has multiple direct text elements")
				}
				directSeen = true
			}
			if !tag.selfClosing {
				inText = true
				captureStart = tag.end + 1
			}
		}

		if !tag.selfClosing {
			stack = append(stack, tag.local)
		}
	}

	if !rootSeen || !rootClosed || len(stack) != 0 || inItem || inRun || inText {
		return nil, xerrors.New("unterminated shared strings XML")
	}
	return shared, nil
}

func hasUnsupportedSharedStringXML(data []byte) bool {
	if bytes.Contains(data, []byte("<![CDATA[")) ||
		bytes.Contains(data, []byte("<!DOCTYPE")) ||
		bytes.Contains(data, []byte("<!--")) {
		return true
	}
	trimmed := bytes.TrimSpace(data)
	trimmed = bytes.TrimPrefix(trimmed, []byte{0xEF, 0xBB, 0xBF})
	if bytes.HasPrefix(trimmed, []byte("<?xml")) {
		if end := bytes.Index(trimmed, []byte("?>")); end >= 0 {
			trimmed = trimmed[end+2:]
		}
	}
	return bytes.Contains(trimmed, []byte("<?"))
}

func decodeXMLText(value []byte) (string, error) {
	if !utf8.Valid(value) || bytes.Contains(value, []byte("]]>")) {
		return "", xerrors.New("invalid UTF-8 in shared string text")
	}
	for remaining := value; len(remaining) > 0; {
		char, size := utf8.DecodeRune(remaining)
		if !validXMLRune(char) {
			return "", xerrors.Newf("invalid XML character %U", char)
		}
		remaining = remaining[size:]
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
				return "", xerrors.New("unterminated entity in shared string text")
			}
			end += offset + 1
			char, err := decodeXMLEntity(value[offset+1 : end])
			if err != nil {
				return "", err
			}
			decoded.WriteRune(char)
			offset = end + 1
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

func decodeXMLEntity(entity []byte) (rune, error) {
	switch {
	case bytes.Equal(entity, []byte("amp")):
		return '&', nil
	case bytes.Equal(entity, []byte("lt")):
		return '<', nil
	case bytes.Equal(entity, []byte("gt")):
		return '>', nil
	case bytes.Equal(entity, []byte("apos")):
		return '\'', nil
	case bytes.Equal(entity, []byte("quot")):
		return '"', nil
	}
	if len(entity) < 2 || entity[0] != '#' {
		return 0, xerrors.Newf("invalid XML entity &%s;", entity)
	}

	base := 10
	number := entity[1:]
	if len(number) > 1 && number[0] == 'x' {
		base = 16
		number = number[1:]
	}
	value, err := strconv.ParseUint(string(number), base, 32)
	if err != nil || !validXMLRune(rune(value)) {
		return 0, xerrors.Newf("invalid XML character reference &%s;", entity)
	}
	return rune(value), nil
}

func validXMLRune(value rune) bool {
	return value == '\t' || value == '\n' || value == '\r' ||
		(value >= 0x20 && value <= 0xD7FF) ||
		(value >= 0xE000 && value <= 0xFFFD) ||
		(value >= 0x10000 && value <= 0x10FFFF)
}
