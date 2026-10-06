package xlsx

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlainXMLTextMatchesStrictDecoder(t *testing.T) {
	texts := []string{
		"", "plain", "\t\n\r", "中文", "\ufffd", "\U00010000", "\U0010ffff",
		"\xff", "\xc0\x80", "\xed\xa0\x80", "\ufffe", "\uffff", "]]>",
	}
	for char := byte(0); char < 0x20; char++ {
		texts = append(texts, string([]byte{char}))
	}
	for _, text := range texts {
		t.Run(fmt.Sprintf("%q", text), func(t *testing.T) {
			decoder := newXMLDecoder(bytes.NewReader([]byte("<t>" + text + "</t>")))
			_, err := nextXMLToken(decoder)
			require.NoError(t, err)
			_, err = readXMLText(decoder)
			require.Equal(t, err == nil, isPlainXMLText([]byte(text)))
		})
	}
}

func TestDecodeXMLTextMatchesStrictDecoder(t *testing.T) {
	for _, text := range []string{"", "plain", "中文\ufffd", "one\r\ntwo\rthree",
		"A&amp;B&#13;", "&nbsp;", "&unknown;", "\xff", "\ufffe", "]]>", "\x00"} {
		t.Run(fmt.Sprintf("%q", text), func(t *testing.T) {
			decoder := newXMLDecoder(bytes.NewReader([]byte("<t>" + text + "</t>")))
			_, err := nextXMLToken(decoder)
			require.NoError(t, err)
			want, strictErr := readXMLText(decoder)
			got, err := decodeXMLText([]byte(text))
			require.Equal(t, strictErr == nil, err == nil)
			if err == nil {
				require.Equal(t, want, got)
			}
		})
	}
}
