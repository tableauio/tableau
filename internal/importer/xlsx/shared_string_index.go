package xlsx

import (
	"bytes"
	"sync"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

type sharedStringSpan struct {
	start, end int
}

// sharedStringIndex retains validated XML spans and decodes only requested
// items. Complex XML uses the existing complete-table decoder instead.
type sharedStringIndex struct {
	mu     sync.Mutex
	data   []byte
	spans  []sharedStringSpan
	values map[int]string
	full   []string
}

func newSharedStringIndex(data []byte) (*sharedStringIndex, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	spans, err := indexSharedStringSpans(data)
	if err != nil {
		full, err := readSharedStrings(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return &sharedStringIndex{full: full}, nil
	}
	return &sharedStringIndex{data: data, spans: spans, values: make(map[int]string)}, nil
}

func (s *sharedStringIndex) value(index int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := len(s.spans)
	if s.full != nil {
		count = len(s.full)
	}
	if index < 0 || index >= count {
		return "", xerrors.Newf("shared string index %d out of range", index)
	}
	if s.full != nil {
		return s.full[index], nil
	}
	if value, ok := s.values[index]; ok {
		return value, nil
	}
	span := s.spans[index]
	decoder := newXMLDecoder(bytes.NewReader(s.data[span.start:span.end]))
	if _, err := nextXMLToken(decoder); err != nil {
		return "", err
	}
	value, err := readSharedString(decoder)
	if err != nil {
		return "", err
	}
	value = decodeEscapes(value)
	s.values[index] = value // An empty value is cached too.
	return value, nil
}

// Index complete items while validating the entire ordinary XML document.
// Reuse the worksheet scanner's tag/text checks; complex or malformed input
// is replayed through the strict shared-string decoder for full diagnostics.
func indexSharedStringSpans(data []byte) ([]sharedStringSpan, error) {
	var spans []sharedStringSpan
	var stack [][]byte
	rootSeen := false
	itemStart := -1
	for offset := 0; offset < len(data); {
		length := bytes.IndexByte(data[offset:], '<')
		if length < 0 {
			length = len(data) - offset
		}
		text := data[offset : offset+length]
		if !isPlainXMLText(text) {
			if _, err := decodeXMLText(text); err != nil {
				return nil, errComplexXML
			}
		}
		offset += length
		if offset == len(data) {
			break
		}
		if offset == 0 && bytes.HasPrefix(data, []byte("<?xml")) {
			end := bytes.Index(data, []byte("?>"))
			if end < 0 {
				return nil, errComplexXML
			}
			end += 2
			decoder := newXMLDecoder(bytes.NewReader(data[:end]))
			if _, err := nextXMLToken(decoder); err != nil {
				return nil, errComplexXML
			}
			offset = end
			continue
		}
		start := offset
		tag, next, err := readPlainXMLTag(data, offset)
		if err != nil {
			return nil, err
		}
		offset = next
		if tag.closing {
			if len(stack) == 0 || !bytes.Equal(stack[len(stack)-1], tag.name) {
				return nil, errComplexXML
			}
			if len(stack) == 2 && itemStart >= 0 {
				spans = append(spans, sharedStringSpan{itemStart, offset})
				itemStart = -1
			}
			stack = stack[:len(stack)-1]
			continue
		}
		if len(stack) == 0 {
			if rootSeen {
				return nil, errComplexXML
			}
			rootSeen = true
		}
		if itemStart >= 0 && (bytes.Equal(tag.name, []byte("si")) ||
			bytes.Equal(stack[len(stack)-1], []byte("t"))) {
			return nil, errComplexXML
		}
		if len(stack) == 1 && bytes.Equal(tag.name, []byte("si")) {
			if tag.empty {
				spans = append(spans, sharedStringSpan{start, offset})
			} else {
				itemStart = start
			}
		}
		if !tag.empty {
			stack = append(stack, tag.name)
		}
	}
	if !rootSeen || len(stack) != 0 {
		return nil, errComplexXML
	}
	return spans, nil
}
