package xlsx

import (
	"encoding/xml"
	"io"
	"strings"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

func readSharedStrings(source io.Reader) ([]string, error) {
	decoder := newXMLDecoder(source)
	var values []string
	var depth int
	var rootSeen bool
	for {
		token, err := nextXMLToken(decoder)
		if err == io.EOF {
			if !rootSeen {
				return nil, xerrors.New("empty shared strings XML")
			}
			return values, nil
		}
		if err != nil {
			return nil, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				if rootSeen {
					return nil, xerrors.New("multiple shared strings root elements")
				}
				rootSeen = true
			}
			if depth == 1 && token.Name.Local == "si" {
				value, err := readSharedString(decoder)
				if err != nil {
					return nil, err
				}
				values = append(values, decodeEscapes(value))
			} else {
				depth++
			}
		case xml.EndElement:
			depth--
		}
	}
}

// readSharedString reads one OOXML string item. Direct text precedes rich-text
// runs; phonetic annotations do not contribute to the displayed value.
func readSharedString(decoder *xml.Decoder) (string, error) {
	var direct, runText string
	var runs strings.Builder
	depth, runDepth := 1, 0
	for {
		token, err := nextXMLToken(decoder)
		if err != nil {
			return "", err
		}
		switch token := token.(type) {
		case xml.StartElement:
			switch {
			case token.Name.Local == "si":
				return "", xerrors.New("nested shared string item")
			case token.Name.Local == "r" && depth == 1:
				runDepth = depth + 1
				runText = ""
			case token.Name.Local == "t" && (depth == 1 || depth == runDepth):
				text, err := readXMLText(decoder)
				if err != nil {
					return "", err
				}
				if runDepth > 0 {
					runText = text
				} else {
					direct = text
				}
				continue
			}
			depth++
		case xml.EndElement:
			if depth == runDepth {
				runs.WriteString(runText)
				runDepth = 0
			}
			depth--
			if depth == 0 {
				return direct + runs.String(), nil
			}
		}
	}
}
