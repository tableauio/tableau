package xlsx

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

var errComplexWorksheetXML = errors.New("worksheet requires the strict XML decoder")

// parseFullWorksheet avoids allocating XML tokens for ordinary OOXML sheets.
// The fast path checks plain tags/attributes and matches opening/closing names.
// Text needing entity handling goes through the standard decoder;
// complex markup is replayed through it as a complete document.
func parseFullWorksheet(data []byte, shared []string) ([][]string, error) {
	rows, err := parsePlainWorksheet(data, shared)
	if errors.Is(err, errComplexWorksheetXML) {
		return parseWorksheet(bytes.NewReader(data), shared, 0)
	}
	return rows, err
}

func parsePlainWorksheet(data []byte, shared []string) ([][]string, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	p := newWorksheetRows(shared, 0)
	var stack [][]byte
	for offset := 0; offset < len(data); {
		start := bytes.IndexByte(data[offset:], '<')
		if start < 0 {
			start = len(data) - offset
		}
		if text := data[offset : offset+start]; !isPlainXMLText(text) {
			if _, err := decodeWorksheetText(text); err != nil {
				// Replay invalid text to retain full-document line diagnostics.
				return nil, errComplexWorksheetXML
			}
		}
		offset += start
		if offset == len(data) {
			break
		}
		// Validate the optional declaration with the standard decoder. It is
		// small and occurs once, unlike cell tags which occur millions of times.
		if len(stack) == 0 && bytes.HasPrefix(data[offset:], []byte("<?xml")) {
			end := bytes.Index(data[offset:], []byte("?>"))
			if end < 0 {
				return nil, errComplexWorksheetXML
			}
			end += offset + 2
			decoder := newXMLDecoder(bytes.NewReader(data[offset:end]))
			if _, err := nextXMLToken(decoder); err != nil {
				return nil, errComplexWorksheetXML
			}
			if _, err := nextXMLToken(decoder); err != io.EOF {
				return nil, errComplexWorksheetXML
			}
			offset = end
			continue
		}
		tag, next, err := readPlainXMLTag(data, offset)
		if err != nil {
			return nil, err
		}
		offset = next
		name := string(tag.name)
		if !tag.closing && p.inCell && (name == "v" ||
			(name == "t" && p.cell.kind == "inlineStr" && p.phoneticDepth == 0)) {
			var text []byte
			if !tag.empty {
				end := bytes.IndexByte(data[offset:], '<')
				if end < 0 {
					return nil, errComplexWorksheetXML
				}
				text = data[offset : offset+end]
				closing, next, err := readPlainXMLTag(data, offset+end)
				if err != nil || !closing.closing || !bytes.Equal(tag.name, closing.name) {
					return nil, errComplexWorksheetXML
				}
				offset = next
			}
			value, err := decodeWorksheetText(text)
			if err != nil {
				return nil, errComplexWorksheetXML
			}
			if name == "v" {
				p.cell.value = value
			} else {
				p.cell.inlineText.WriteString(value)
			}
			continue
		}
		if tag.closing {
			if len(stack) == 0 || !bytes.Equal(stack[len(stack)-1], tag.name) {
				return nil, errComplexWorksheetXML
			}
			stack = stack[:len(stack)-1]
		} else {
			if !tag.empty {
				stack = append(stack, tag.name)
			}
			switch name {
			case "row":
				if _, err := p.startRow(string(tag.reference), tag.hasReference); err != nil {
					return nil, err
				}
			case "c":
				if p.inRow {
					if err := p.startCell(string(tag.reference), string(tag.kind), tag.hasReference); err != nil {
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
			}
		}
		if tag.closing || tag.empty {
			switch name {
			case "row":
				p.endRow()
			case "c":
				if p.inCell {
					if err := p.endCell(); err != nil {
						return nil, err
					}
				}
			case "rPh":
				if p.phoneticDepth > 0 {
					p.phoneticDepth--
				}
			}
		}
	}
	if len(stack) != 0 {
		return nil, errComplexWorksheetXML
	}
	return p.rows, nil
}

type plainXMLTag struct {
	name, reference, kind []byte
	closing, empty        bool
	hasReference          bool
}

func readPlainXMLTag(data []byte, offset int) (plainXMLTag, int, error) {
	var tag plainXMLTag
	start := offset + 1
	if start < len(data) && data[start] == '/' {
		tag.closing = true
		start++
	}
	end := start
	for end < len(data) && isASCIINameByte(data[end], end == start) {
		end++
	}
	if end == start {
		return tag, offset, errComplexWorksheetXML
	}
	tag.name = data[start:end]
	var hasKind bool
	for end < len(data) {
		beforeSpace := end
		for end < len(data) && isXMLSpace(data[end]) {
			end++
		}
		if end < len(data) && data[end] == '>' {
			return tag, end + 1, nil
		}
		if !tag.closing && end+1 < len(data) && data[end] == '/' && data[end+1] == '>' {
			tag.empty = true
			return tag, end + 2, nil
		}
		if tag.closing || end == beforeSpace {
			break
		}
		start = end
		for end < len(data) && isASCIINameByte(data[end], end == start) {
			end++
		}
		if end == start {
			break
		}
		prefix := data[start:end]
		if end < len(data) && data[end] == ':' {
			end++
			local := end
			for end < len(data) && isASCIINameByte(data[end], end == local) {
				end++
			}
			if end == local {
				break
			}
		}
		key := data[start:end]
		for end < len(data) && isXMLSpace(data[end]) {
			end++
		}
		if end == len(data) || data[end] != '=' {
			break
		}
		end++
		for end < len(data) && isXMLSpace(data[end]) {
			end++
		}
		if end == len(data) || (data[end] != '\'' && data[end] != '"') {
			break
		}
		quote := data[end]
		end++
		length := bytes.IndexByte(data[end:], quote)
		if length < 0 {
			break
		}
		value := data[end : end+length]
		if !isPlainXMLText(value) || bytes.ContainsAny(value, "<\n\r\t") {
			break
		}
		end += length + 1
		if !bytes.Equal(prefix, []byte("xmlns")) {
			if colon := bytes.IndexByte(key, ':'); colon >= 0 {
				key = key[colon+1:]
				// Resolving qualified row/cell attributes needs namespace scope.
				// Ordinary OOXML uses unqualified r/t; leave the rest to SAX.
				if bytes.Equal(key, []byte("r")) || bytes.Equal(key, []byte("t")) {
					return tag, offset, errComplexWorksheetXML
				}
			}
			switch string(key) {
			case "r":
				if !tag.hasReference {
					tag.reference, tag.hasReference = value, true
				}
			case "t":
				if !hasKind {
					tag.kind, hasKind = value, true
				}
			}
		}
	}
	return tag, offset, errComplexWorksheetXML
}

func isASCIINameByte(char byte, first bool) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char == '_' ||
		!first && (char >= '0' && char <= '9' || char == '-' || char == '.')
}

func isXMLSpace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r'
}

func isPlainXMLText(text []byte) bool {
	if bytes.Contains(text, []byte("]]>")) {
		return false
	}
	for offset := 0; offset < len(text); {
		char := text[offset]
		if char == '&' || char < 0x20 && !isXMLSpace(char) {
			return false
		}
		if char < utf8.RuneSelf {
			offset++
			continue
		}
		r, size := utf8.DecodeRune(text[offset:])
		// Valid UTF-8 excludes surrogates and out-of-range code points. The
		// remaining non-ASCII characters forbidden by XML are U+FFFE/U+FFFF.
		if size == 1 || r == '\ufffe' || r == '\uffff' {
			return false
		}
		offset += size
	}
	return true
}

func normalizeXMLNewlines(text []byte) string {
	value := string(text)
	if bytes.IndexByte(text, '\r') < 0 {
		return value
	}
	return strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
}

// Decode exceptional text in isolation so one formula containing an entity
// does not send an otherwise ordinary, million-cell sheet through SAX again.
func decodeWorksheetText(text []byte) (string, error) {
	if isPlainXMLText(text) {
		return normalizeXMLNewlines(text), nil
	}
	decoder := newXMLDecoder(bytes.NewReader(text))
	token, err := nextXMLToken(decoder)
	if err != nil {
		return "", err
	}
	value, ok := token.(xml.CharData)
	if !ok {
		return "", errComplexWorksheetXML
	}
	result := string(value)
	if _, err := nextXMLToken(decoder); err != io.EOF {
		if err == nil {
			err = errComplexWorksheetXML
		}
		return "", err
	}
	return result, nil
}
