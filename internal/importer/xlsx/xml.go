package xlsx

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// newXMLDecoder uses strict XML parsing with common named HTML entities,
// including &nbsp;. UTF-16 is transcoded before parsing; other declared
// encodings request the compatibility reader. The standard library handles
// CDATA, comments, namespace prefixes, and character references.
func newXMLDecoder(source io.Reader) *xml.Decoder {
	var prefix [4]byte
	// Buffered text fragments already support ReadAt; avoid allocating a
	// second buffer for each entity-bearing cell in the full-sheet decoder.
	switch reader := source.(type) {
	case *bytes.Reader:
		_, _ = reader.ReadAt(prefix[:], reader.Size()-int64(reader.Len()))
	case *strings.Reader:
		_, _ = reader.ReadAt(prefix[:], reader.Size()-int64(reader.Len()))
	default:
		buffered := bufio.NewReader(source)
		data, _ := buffered.Peek(len(prefix))
		copy(prefix[:], data)
		source = buffered
	}
	var utf16 bool
	// XML's BOM and initial '<' byte patterns identify UTF-16 before the
	// standard decoder sees any bytes. CharsetReader alone cannot do that.
	switch {
	case bytes.HasPrefix(prefix[:], []byte{0xff, 0xfe}), bytes.HasPrefix(prefix[:], []byte{'<', 0, '?', 0}):
		source = transform.NewReader(source, unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewDecoder())
		utf16 = true
	case bytes.HasPrefix(prefix[:], []byte{0xfe, 0xff}), bytes.HasPrefix(prefix[:], []byte{0, '<', 0, '?'}):
		source = transform.NewReader(source, unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewDecoder())
		utf16 = true
	}
	decoder := xml.NewDecoder(source)
	decoder.Entity = xml.HTMLEntity
	decoder.CharsetReader = unsupportedXMLCharset
	if utf16 {
		decoder.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
			if strings.EqualFold(charset, "UTF-16") ||
				strings.EqualFold(charset, "UTF-16LE") || strings.EqualFold(charset, "UTF-16BE") {
				return input, nil // Already transcoded before XML parsing.
			}
			return unsupportedXMLCharset(charset, input)
		}
	}
	return decoder
}

func unsupportedXMLCharset(charset string, _ io.Reader) (io.Reader, error) {
	return nil, fmt.Errorf("%w: XML encoding %q", ErrUnsupported, charset)
}

func nextXMLToken(decoder *xml.Decoder) (xml.Token, error) {
	token, err := decoder.Token()
	if directive, ok := token.(xml.Directive); ok &&
		bytes.HasPrefix(bytes.TrimSpace(directive), []byte("DOCTYPE")) {
		return nil, fmt.Errorf("%w: XML document type declaration", ErrUnsupported)
	}
	return token, err
}

// readXMLText consumes one text element, preserving whitespace and joining
// character data across CDATA, comments, and processing instructions.
func readXMLText(decoder *xml.Decoder) (string, error) {
	var text strings.Builder
	for {
		token, err := nextXMLToken(decoder)
		if err != nil {
			return "", err
		}
		switch token := token.(type) {
		case xml.CharData:
			text.Write(token)
		case xml.EndElement:
			return text.String(), nil
		case xml.StartElement:
			return "", fmt.Errorf("nested XML element in text: %s", token.Name.Local)
		}
	}
}

func xmlAttribute(element xml.StartElement, name string) (string, bool) {
	for _, attribute := range element.Attr {
		if attribute.Name.Local == name && attribute.Name.Space != "xmlns" {
			return attribute.Value, true
		}
	}
	return "", false
}
