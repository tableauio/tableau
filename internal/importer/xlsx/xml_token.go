package xlsx

import (
	"bytes"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

type xmlTag struct {
	local       []byte
	attributes  []byte
	start       int
	end         int
	closing     bool
	selfClosing bool
}

func nextXMLTag(data []byte, offset int) (xmlTag, int, bool, error) {
	for offset < len(data) {
		relative := bytes.IndexByte(data[offset:], '<')
		if relative < 0 {
			return xmlTag{}, len(data), false, nil
		}
		start := offset + relative
		if bytes.HasPrefix(data[start:], []byte("<!--")) {
			end := bytes.Index(data[start+4:], []byte("-->"))
			if end < 0 {
				return xmlTag{}, len(data), false, xerrors.Newf("unterminated XML comment at byte %d", start)
			}
			offset = start + 4 + end + 3
			continue
		}
		if bytes.HasPrefix(data[start:], []byte("<?")) {
			end := bytes.Index(data[start+2:], []byte("?>"))
			if end < 0 {
				return xmlTag{}, len(data), false, xerrors.Newf("unterminated XML processing instruction at byte %d", start)
			}
			offset = start + 2 + end + 2
			continue
		}
		position := start + 1
		closing := false
		if position < len(data) && data[position] == '/' {
			closing = true
			position++
		}
		for position < len(data) && isXMLSpace(data[position]) {
			position++
		}
		nameStart := position
		for position < len(data) && !isXMLSpace(data[position]) && data[position] != '/' && data[position] != '>' {
			position++
		}
		if nameStart == position || data[nameStart] == '!' {
			end := bytes.IndexByte(data[position:], '>')
			if end < 0 {
				return xmlTag{}, len(data), false, xerrors.Newf("unterminated XML declaration at byte %d", start)
			}
			offset = position + end + 1
			continue
		}
		name := data[nameStart:position]
		localName := name
		if colon := bytes.LastIndexByte(name, ':'); colon >= 0 {
			localName = name[colon+1:]
		}
		quote := byte(0)
		end := position
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
			return xmlTag{}, len(data), false, xerrors.Newf("unterminated XML tag at byte %d", start)
		}
		last := end - 1
		for last >= position && isXMLSpace(data[last]) {
			last--
		}
		return xmlTag{
			local:       localName,
			attributes:  data[position:end],
			start:       start,
			end:         end,
			closing:     closing,
			selfClosing: last >= position && data[last] == '/',
		}, end + 1, true, nil
	}
	return xmlTag{}, len(data), false, nil
}

func xmlAttribute(attributes []byte, name byte) ([]byte, bool) {
	for offset := 0; offset < len(attributes); {
		for offset < len(attributes) && (isXMLSpace(attributes[offset]) || attributes[offset] == '/') {
			offset++
		}
		start := offset
		for offset < len(attributes) && !isXMLSpace(attributes[offset]) && attributes[offset] != '=' && attributes[offset] != '/' {
			offset++
		}
		if start == offset {
			break
		}
		key := attributes[start:offset]
		if colon := bytes.LastIndexByte(key, ':'); colon >= 0 {
			if bytes.Equal(key[:colon], []byte("xmlns")) {
				key = nil
			} else {
				key = key[colon+1:]
			}
		}
		for offset < len(attributes) && isXMLSpace(attributes[offset]) {
			offset++
		}
		if offset >= len(attributes) || attributes[offset] != '=' {
			continue
		}
		offset++
		for offset < len(attributes) && isXMLSpace(attributes[offset]) {
			offset++
		}
		if offset >= len(attributes) || (attributes[offset] != '\'' && attributes[offset] != '"') {
			continue
		}
		quote := attributes[offset]
		offset++
		valueStart := offset
		for offset < len(attributes) && attributes[offset] != quote {
			offset++
		}
		value := attributes[valueStart:offset]
		if offset < len(attributes) {
			offset++
		}
		if len(key) == 1 && key[0] == name {
			return value, true
		}
	}
	return nil, false
}

// plainXMLAttribute rejects character references in worksheet coordinates and
// cell types. Falling back to the compatibility reader is safer than silently
// interpreting the undecoded bytes with different semantics.
func plainXMLAttribute(attributes []byte, name byte) ([]byte, bool, error) {
	value, ok := xmlAttribute(attributes, name)
	if ok && bytes.IndexByte(value, '&') >= 0 {
		return nil, false, xerrors.Newf("unsupported XML character reference in worksheet attribute %q", name)
	}
	return value, ok, nil
}

func isXMLSpace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\r' || char == '\n'
}
