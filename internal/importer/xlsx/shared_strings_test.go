package xlsx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseSharedStrings(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<x:sst xmlns:x="urn:spreadsheet">
  <x:si><x:t xml:space="preserve"> one &amp; two&#13;` + "\r\n" + `three </x:t></x:si>
  <x:si>
    <x:r><x:rPr/><x:t>_x00</x:t></x:r>
    <x:rPh><x:t>ignored</x:t></x:rPh>
    <x:r><x:t>41_</x:t></x:r>
  </x:si>
  <x:si><x:r><x:t>A</x:t></x:r><x:r/><x:r><x:t>B</x:t></x:r></x:si>
  <x:si/>
</x:sst>`)

	shared, err := parseSharedStrings(data)
	require.NoError(t, err)
	require.Equal(t, []string{" one & two\r\nthree ", "A", "AB", ""}, shared)
}

func TestParseSharedStringsRejectsUnsafeInput(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{name: "empty", xml: ""},
		{name: "unterminated", xml: `<sst><si>`},
		{name: "mismatched", xml: `<sst><si></sst>`},
		{name: "multiple roots", xml: `<sst/><sst/>`},
		{name: "mixed direct and rich text", xml: `<sst><si><t>A</t><r><t>B</t></r></si></sst>`},
		{name: "multiple direct text", xml: `<sst><si><t>A</t><t>B</t></si></sst>`},
		{name: "unknown entity", xml: `<sst><si><t>&nbsp;</t></si></sst>`},
		{name: "invalid character reference", xml: `<sst><si><t>&#0;</t></si></sst>`},
		{name: "raw NUL", xml: "<sst><si><t>\x00</t></si></sst>"},
		{name: "raw control", xml: "<sst><si><t>\x01</t></si></sst>"},
		{name: "invalid code point FFFE", xml: "<sst><si><t>\uFFFE</t></si></sst>"},
		{name: "invalid code point FFFF", xml: "<sst><si><t>\uFFFF</t></si></sst>"},
		{name: "forbidden text terminator", xml: `<sst><si><t>a]]>b</t></si></sst>`},
		{name: "CDATA", xml: `<sst><si><t><![CDATA[A]]></t></si></sst>`},
		{name: "comment", xml: `<sst><!-- comment --></sst>`},
		{name: "processing instruction", xml: `<sst><?custom value?></sst>`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseSharedStrings([]byte(test.xml))
			require.Error(t, err)
		})
	}
}

func TestParseSharedStringsAcceptsEmptyRoot(t *testing.T) {
	shared, err := parseSharedStrings([]byte("\xEF\xBB\xBF<?xml version=\"1.0\"?><sst/>"))
	require.NoError(t, err)
	require.Empty(t, shared)
}

func TestDecodeXMLTextUsesXMLCharacterReferences(t *testing.T) {
	text, err := decodeXMLText([]byte("&lt;&amp;&gt;&apos;&quot;&#128;&#xD;"))
	require.NoError(t, err)
	require.Equal(t, "<&>'\"\u0080\r", text)
}
