package xlsx

import (
	"bytes"
	"errors"
	"io"
)

// parseFullWorksheet avoids allocating XML tokens for ordinary OOXML sheets.
// The fast path checks plain tags/attributes and matches opening/closing names.
// Text needing entity handling goes through the standard decoder;
// complex markup is replayed through it as a complete document.
func parseFullWorksheet(data []byte, shared []string) ([][]string, error) {
	return parseFullWorksheetWithRows(data, newWorksheetRows(shared, 0))
}

func parseFullWorksheetWithRows(data []byte, p *worksheetRows) ([][]string, error) {
	rows, err := parsePlainWorksheetWithRows(data, p)
	if errors.Is(err, errComplexXML) {
		fresh := newWorksheetRows(p.shared, 0)
		fresh.sharedValue = p.sharedValue
		return parseWorksheetWithRows(bytes.NewReader(data), fresh)
	}
	return rows, err
}

func parsePlainWorksheet(data []byte, shared []string) ([][]string, error) {
	// This decoder only handles full reads. With rowLimit == 0, startRow
	// cannot request an early stop; bounded reads use parseWorksheet instead.
	return parsePlainWorksheetWithRows(data, newWorksheetRows(shared, 0))
}

func parsePlainWorksheetWithRows(data []byte, p *worksheetRows) ([][]string, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var stack [][]byte
	for offset := 0; offset < len(data); {
		start := bytes.IndexByte(data[offset:], '<')
		if start < 0 {
			start = len(data) - offset
		}
		if text := data[offset : offset+start]; !isPlainXMLText(text) {
			if _, err := decodeXMLText(text); err != nil {
				// Replay invalid text to retain full-document line diagnostics.
				return nil, errComplexXML
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
				return nil, errComplexXML
			}
			end += offset + 2
			decoder := newXMLDecoder(bytes.NewReader(data[offset:end]))
			if _, err := nextXMLToken(decoder); err != nil {
				return nil, errComplexXML
			}
			if _, err := nextXMLToken(decoder); err != io.EOF {
				return nil, errComplexXML
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
					return nil, errComplexXML
				}
				text = data[offset : offset+end]
				closing, next, err := readPlainXMLTag(data, offset+end)
				if err != nil || !closing.closing || !bytes.Equal(tag.name, closing.name) {
					return nil, errComplexXML
				}
				offset = next
			}
			value, err := decodeXMLText(text)
			if err != nil {
				return nil, errComplexXML
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
				return nil, errComplexXML
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
		return nil, errComplexXML
	}
	return p.rows, nil
}
