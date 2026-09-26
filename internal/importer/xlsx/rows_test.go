package xlsx

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRows(t *testing.T) {
	data := []byte(`<worksheet><sheetData>
<row r="2"><c r="B2" t="s"><v>0</v></c><c r="C2" t="inlineStr"><is><t>A&amp;B</t></is></c><c r="D2"><f>1+1</f><v></v></c></row>
<row r="4"><c r="A4"><v>42</v></c></row>
</sheetData></worksheet>`)

	rows, err := parseRows(data, []string{"shared"})
	require.NoError(t, err)
	require.Equal(t, [][]string{
		nil,
		{"", "shared", "A&B", ""},
		nil,
		{"42"},
	}, rows)
}

func TestParseRowsHandlesSparseAndRichCells(t *testing.T) {
	data := []byte(`<x:worksheet><x:sheetData>
<x:row r='1'><x:c r='$A$1' t='inlineStr'><x:is><x:r><x:t>Hi</x:t></x:r><x:rPh><x:t>ignored</x:t></x:rPh><x:r><x:t>!</x:t></x:r></x:is></x:c><x:c r='C1'/><x:c r='D1' t='s'><x:v>9</x:v></x:c></x:row>
<x:row r='2'/><x:row r='3'><x:c><x:v>7</x:v></x:c></x:row>
</x:sheetData></x:worksheet>`)

	rows, err := parseRows(data, nil)
	require.NoError(t, err)
	require.Equal(t, [][]string{
		{"Hi!", "", "", "9"},
		nil,
		{"7"},
	}, rows)
}

func TestParseRowsRejectsMalformedXML(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{name: "comment", xml: "<!--"},
		{name: "processing instruction", xml: "<?xml"},
		{name: "tag", xml: "<row"},
		{name: "cell", xml: "<row><c><v>1</v></row>"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseRows([]byte(test.xml), nil)
			require.Error(t, err)
		})
	}
}

func TestParseRowsRejectsInvalidCoordinates(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{name: "zero row", xml: `<row r="0"></row>`},
		{name: "row overflow", xml: `<row r="1048577"></row>`},
		{name: "invalid cell", xml: `<row r="1"><c r="1"><v>x</v></c></row>`},
		{name: "column overflow", xml: `<row r="1"><c r="XFE1"><v>x</v></c></row>`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseRows([]byte(test.xml), nil)
			require.Error(t, err)
		})
	}
}

func TestAttribute(t *testing.T) {
	attrs := []byte(` xmlns:r="urn:test" r:id='rId1' t = "inlineStr" broken value=noquote`)
	value, ok := attribute(attrs, 't')
	require.True(t, ok)
	require.Equal(t, "inlineStr", string(value))
	value, ok = attribute(attrs, 'r')
	require.False(t, ok)
	require.Nil(t, value)
}

func TestParseColumn(t *testing.T) {
	require.Equal(t, 1, parseColumn([]byte("A1")))
	require.Equal(t, 16384, parseColumn([]byte("$XFD$1")))
	require.Equal(t, 28, parseColumn([]byte("ab12")))
	require.Zero(t, parseColumn([]byte("1")))
	require.Zero(t, parseColumn([]byte(strings.Repeat("Z", 32))))
}

func TestParseInt(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	value, err := parseInt([]byte(strconv.Itoa(maxInt)))
	require.NoError(t, err)
	require.Equal(t, maxInt, value)

	_, err = parseInt(nil)
	require.ErrorIs(t, err, strconv.ErrSyntax)
	_, err = parseInt([]byte("12x"))
	require.ErrorIs(t, err, strconv.ErrSyntax)
	_, err = parseInt([]byte(strconv.Itoa(maxInt) + "0"))
	require.ErrorIs(t, err, strconv.ErrRange)
}

func TestDecodeEscapes(t *testing.T) {
	require.Equal(t, "A", decodeEscapes("_x0041_"))
	require.Equal(t, "_x0041_", decodeEscapes("_x005F_x0041_"))
	require.Equal(t, "😀", decodeEscapes("_xD83D__xDE00_"))
	require.Equal(t, "bad_xZZZZ_", decodeEscapes("bad_xZZZZ_"))
	require.Equal(t, "plain", decodeEscapes("plain"))
}
