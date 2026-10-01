package xlsx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
)

// newXMLDecoder uses strict XML parsing with common named HTML entities,
// including &nbsp;. The standard library handles CDATA, comments, namespace
// prefixes, and character references.
func newXMLDecoder(source io.Reader) *xml.Decoder {
	decoder := xml.NewDecoder(source)
	decoder.Entity = xml.HTMLEntity
	return decoder
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
	var text []byte
	for {
		token, err := nextXMLToken(decoder)
		if err != nil {
			return "", err
		}
		switch token := token.(type) {
		case xml.CharData:
			text = append(text, token...)
		case xml.EndElement:
			return string(text), nil
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
