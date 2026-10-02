package xlsx

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

var errComplexXML = errors.New("XML requires the strict decoder")

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
		return tag, offset, errComplexXML
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
					return tag, offset, errComplexXML
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
	return tag, offset, errComplexXML
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
func decodeXMLText(text []byte) (string, error) {
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
		return "", errComplexXML
	}
	result := string(value)
	if _, err := nextXMLToken(decoder); err != io.EOF {
		if err == nil {
			err = errComplexXML
		}
		return "", err
	}
	return result, nil
}
