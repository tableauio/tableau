package xlsx

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFullWorksheetMatchesStrictDecoder(t *testing.T) {
	for _, content := range []string{
		`<row r="1"><c r="B1" t="s"><v>0</v></c><c r="B1"><v>2</v></c><c><f/><v/></c></row><row r="1"><c r="A1"><v>3</v></c></row>`,
		`<row r="2"><c r="A2" t="inlineStr"><is><r><t>Hi</t></r><rPh><t>ignore</t></rPh><r><t>!</t></r></is></c></row><row r="3"/>`,
		`<row r="1"><c r="A1" t="inlineStr"><is><t>中文</t></is></c></row>`,
		`<row r="1"><c r="A1" t="inlineStr"><is><t>A&amp;B&#13;</t></is></c></row>`,
		`<row r="1"><c r="A1" t="inlineStr"><is><t><![CDATA[A<&B]]></t></is></c></row>`,
		`<row r="1"><c r="A1" t="s"><v><!--index-->0<?hint value?></v></c></row>`,
		`<row r="1"><c r="A1" t="&#x73;"><v>0</v></c></row>`,
		`<row r="1" xmlns:x="urn:test" x:r="2"><c x:r="B1"><v>2</v></c></row>`,
		`<row r="1" xmlns:x="xmlns"><c x:r="B1"><v>2</v></c></row>`,
		`<row r="1"><c :r="A1"><v>2</v></c></row>`,
		`<row r="1"><c r="A1"t="s"><v>0</v></c></row>`,
		`<row r="1"><c r="A1"><v>1</v></row>`,
		`<row r="1"><c r="A1" t="inlineStr"><is><t>a]]>b</t></is></c></row>`,
		`<row r="1"><c r="A1"><v>0</v missing></c></row>`,
		`<row r="1"><c r="A1"><v>0</v/></c></row>`,
	} {
		data := []byte(`<?xml version="1.0" encoding="UTF-8"?><worksheet><sheetData>` + content + `</sheetData></worksheet>`)
		got, err := parseFullWorksheet(data, []string{"shared"})
		want, strictErr := parseWorksheet(bytes.NewReader(data), []string{"shared"}, 0)
		require.Equal(t, strictErr == nil, err == nil, "XML: %s", content)
		if err == nil {
			require.Equal(t, want, got, "XML: %s", content)
		}
	}
}

func TestPlainWorksheetAcceptsExportedXML(t *testing.T) {
	data := []byte("\xef\xbb\xbf<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\r\n" +
		`<worksheet xmlns="urn:sheet" xmlns:r="urn:rel" xmlns:x14ac="urn:layout"><sheetData>` +
		"<row r=\"1\" spans=\"1:3\" x14ac:dyDescent=\"0.2\"><c r=\"A1\" t=\"s\"><v>0</v></c>" +
		"<c r=\"C1\" t=\"str\"><v>one\r\ntwo\rthree&#13;中文</v></c></row></sheetData></worksheet>")
	got, err := parsePlainWorksheet(data, []string{"shared"})
	require.NoError(t, err)
	want, err := parseWorksheet(bytes.NewReader(data), []string{"shared"}, 0)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestFullWorksheetPreservesXMLSyntaxErrors(t *testing.T) {
	for _, text := range []string{"&missing;", "&#0;", "\x00", "\xff", "\xed\xa0\x80", "\ufffe", "\uffff", "]]>"} {
		data := []byte("<worksheet>\n<sheetData>\n<row r=\"1\"><c><v>" + text + "</v></c></row>\n</sheetData></worksheet>")
		_, err := parseFullWorksheet(data, nil)
		_, strictErr := parseWorksheet(bytes.NewReader(data), nil, 0)
		require.Error(t, strictErr)
		require.EqualError(t, err, strictErr.Error())
	}
}

func FuzzFullWorksheetMatchesStrictDecoder(f *testing.F) {
	for _, seed := range []string{
		`<worksheet/>`,
		`<worksheet><sheetData><row r="1"><c r="B1" t="s"><v>0</v></c><c><v>2</v></c></row></sheetData></worksheet>`,
		`<worksheet><sheetData><row r="1"><c t="inlineStr"><is><r><t>A</t></r><rPh><t>B</t></rPh><r><t>C</t></r></is></c></row></sheetData></worksheet>`,
		`<?xml version="1.0"?><worksheet xmlns="urn:test" xmlns:r="urn:rel"><sheetData><row r="3"/></sheetData></worksheet>`,
		"<?xml version=\"1.0\"?>\r\n<worksheet><sheetData><row r=\"1\"><c><v>1\r\n2\r3</v></c></row></sheetData></worksheet>",
		`<worksheet><sheetData><row r="1"><c r="A1" t="str"><v>&nbsp;</v></c></row></sheetData></worksheet>`,
		`<worksheet><sheetData><row r="1"><c t="inlineStr"><is><t>中文\ufffd</t></is></c></row></sheetData></worksheet>`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64<<10 {
			t.Skip()
		}
		got, err := parseFullWorksheet(data, []string{"shared"})
		want, strictErr := parseWorksheet(bytes.NewReader(data), []string{"shared"}, 0)
		if (err == nil) != (strictErr == nil) || err == nil && !reflect.DeepEqual(got, want) {
			t.Fatalf("fast result %v (%v), strict result %v (%v)", got, err, want, strictErr)
		}
	})
}

// A loose allocation ceiling catches accidental whole-sheet SAX fallback
// without depending on machine speed or a particular compiler's exact count.
func TestFullWorksheetAllocationBudget(t *testing.T) {
	const rows, columns = 32, 64
	data := benchmarkWorksheet(rows, columns)
	for _, text := range []string{"", "中文\ufffd"} {
		t.Run(fmt.Sprintf("text=%q", text), func(t *testing.T) {
			input := data
			if text != "" {
				input = inlineBenchmarkWorksheet(data, text)
			}
			allocations := testing.AllocsPerRun(3, func() {
				_, err := parseFullWorksheet(input, []string{"value"})
				if err != nil {
					t.Fatal(err)
				}
			})
			require.Less(t, allocations, float64(rows*columns*5))
		})
	}
}

func benchmarkWorksheet(rows, columns int) []byte {
	var input bytes.Buffer
	input.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\r\n")
	input.WriteString(`<worksheet xmlns="urn:sheet" xmlns:r="urn:rel" xmlns:x14ac="urn:layout"><sheetData>`)
	for row := 1; row <= rows; row++ {
		fmt.Fprintf(&input, `<row r="%d" spans="1:%d" x14ac:dyDescent="0.2">`, row, columns)
		for column := 0; column < columns; column++ {
			input.WriteString(`<c t="s"><v>0</v></c>`)
		}
		input.WriteString(`</row>`)
	}
	input.WriteString(`</sheetData></worksheet>`)
	return input.Bytes()
}

func inlineBenchmarkWorksheet(data []byte, text string) []byte {
	return bytes.ReplaceAll(data, []byte(`<c t="s"><v>0</v></c>`),
		[]byte(`<c t="inlineStr"><is><t>`+text+`</t></is></c>`))
}

func BenchmarkFullWorksheet(b *testing.B) {
	data := benchmarkWorksheet(512, 64)
	shared := []string{"value"}
	for _, input := range []struct {
		name string
		data []byte
	}{
		{"shared", data},
		{"unicode", inlineBenchmarkWorksheet(data, "中文\ufffd")},
		{"entities", inlineBenchmarkWorksheet(data, "A&amp;B&#13;")},
	} {
		b.Run(input.name, func(b *testing.B) {
			for _, decoder := range []string{"strict", "fast"} {
				b.Run(decoder, func(b *testing.B) {
					b.ReportAllocs()
					b.SetBytes(int64(len(input.data)))
					for i := 0; i < b.N; i++ {
						var err error
						if decoder == "fast" {
							_, err = parseFullWorksheet(input.data, shared)
						} else {
							_, err = parseWorksheet(bytes.NewReader(input.data), shared, 0)
						}
						if err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		})
	}
}
