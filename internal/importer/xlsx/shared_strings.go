package xlsx

import (
	"bytes"
	"strings"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

// parseSharedStrings decodes the small OOXML subset used by sharedStrings.xml
// without building an XML object tree. Unsupported constructs return an error
// so callers can retry the workbook with the compatibility reader.
func parseSharedStrings(data []byte) ([]string, error) {
	if hasUnsupportedXML(data) {
		return nil, xerrors.New("unsupported XML construct in shared strings")
	}

	var (
		sharedStrings []string
		elementStack  [][]byte
		offset        int
		rootSeen      bool
		rootClosed    bool

		inItem     bool
		itemDepth  int
		directSeen bool
		hasRuns    bool
		directText string
		itemText   strings.Builder

		inRun       bool
		runDepth    int
		runTextSeen bool
		runText     strings.Builder

		inText       bool
		captureStart int
	)

	for {
		element, nextOffset, ok, err := nextXMLTag(data, offset)
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}
		offset = nextOffset

		if element.closing {
			if len(elementStack) == 0 || !bytes.Equal(elementStack[len(elementStack)-1], element.local) {
				return nil, xerrors.Newf("mismatched shared strings closing tag %q", element.local)
			}
			depth := len(elementStack)
			if depth == 1 {
				rootClosed = true
			}
			switch {
			case inText && bytes.Equal(element.local, []byte("t")):
				text, err := decodeXMLText(data[captureStart:element.start])
				if err != nil {
					return nil, err
				}
				if inRun {
					runText.WriteString(text)
				} else {
					directText = text
				}
				inText = false
			case inRun && depth == runDepth && bytes.Equal(element.local, []byte("r")):
				itemText.WriteString(runText.String())
				runText.Reset()
				inRun = false
				runDepth = 0
				runTextSeen = false
			case inItem && depth == itemDepth && bytes.Equal(element.local, []byte("si")):
				if directSeen && hasRuns {
					return nil, xerrors.New("shared string mixes direct text and rich-text runs")
				}
				value := itemText.String()
				if directSeen {
					value = directText
				}
				sharedStrings = append(sharedStrings, decodeEscapes(value))
				itemText.Reset()
				directText = ""
				directSeen = false
				hasRuns = false
				inItem = false
				itemDepth = 0
			}
			elementStack = elementStack[:len(elementStack)-1]
			continue
		}

		if inText {
			return nil, xerrors.New("nested XML element in shared string text")
		}

		parentDepth := len(elementStack)
		if parentDepth == 0 {
			if rootSeen {
				return nil, xerrors.New("multiple shared strings root elements")
			}
			rootSeen = true
			rootClosed = element.selfClosing
		}
		switch {
		case !inItem && parentDepth == 1 && bytes.Equal(element.local, []byte("si")):
			if element.selfClosing {
				sharedStrings = append(sharedStrings, "")
				continue
			}
			inItem = true
			itemDepth = parentDepth + 1
			directSeen = false
			hasRuns = false
			directText = ""
		case inItem && parentDepth == itemDepth && bytes.Equal(element.local, []byte("r")):
			if element.selfClosing {
				hasRuns = true
				continue
			}
			inRun = true
			runDepth = parentDepth + 1
			runTextSeen = false
			hasRuns = true
		case inItem && bytes.Equal(element.local, []byte("t")) &&
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
			if !element.selfClosing {
				inText = true
				captureStart = element.end + 1
			}
		}

		if !element.selfClosing {
			elementStack = append(elementStack, element.local)
		}
	}

	if !rootSeen || !rootClosed || len(elementStack) != 0 || inItem || inRun || inText {
		return nil, xerrors.New("unterminated shared strings XML")
	}
	return sharedStrings, nil
}
