package xlsx

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSharedStringIndexDecodesOnlyRequestedItems(t *testing.T) {
	data := benchmarkSharedStrings(50_000)
	index, err := newSharedStringIndex(data)
	require.NoError(t, err)
	require.Len(t, index.spans, 50_000)
	require.Empty(t, index.values)
	for _, id := range []int{49_999, 3, 49_999} {
		value, err := index.value(id)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("value-%d", id), value)
	}
	require.Len(t, index.values, 2)
	_, err = index.value(50_000)
	require.ErrorContains(t, err, "out of range")
}

func TestSharedStringIndexPreservesRichTextAndEmptyValues(t *testing.T) {
	data := []byte(`<sst><si/><si><r><t>_x00</t></r><rPh><t>ignored</t></rPh><r><t>41_</t></r></si><si><t xml:space="preserve"> 中文 &amp; &#13; </t></si></sst>`)
	index, err := newSharedStringIndex(data)
	require.NoError(t, err)
	require.Len(t, index.spans, 3)
	want, err := readSharedStrings(bytes.NewReader(data))
	require.NoError(t, err)
	for id, value := range want {
		got, err := index.value(id)
		require.NoError(t, err)
		require.Equal(t, value, got)
	}
	_, err = index.value(0)
	require.NoError(t, err)
	require.Len(t, index.values, 3)
}

func TestSharedStringIndexValidatesUnusedItems(t *testing.T) {
	for _, tail := range []string{`<si><t>&unknown;</t></si>`, `<si><t><b/></t></si>`,
		`<si><si/></si>`, `<si></t>`, "<si><t>\x00</t></si>"} {
		_, err := newSharedStringIndex([]byte(`<sst><si><t>header</t></si>` + tail + `</sst>`))
		require.Error(t, err, "tail: %q", tail)
	}
}

func TestSharedStringIndexUsesStrictDecoderForComplexXML(t *testing.T) {
	for _, data := range []string{
		`<x:sst xmlns:x="urn:sheet"><x:si><x:t>value</x:t></x:si></x:sst>`,
		`<sst><!-- comment --><si><t><![CDATA[A<&B]]></t></si></sst>`,
	} {
		index, err := newSharedStringIndex([]byte(data))
		require.NoError(t, err)
		require.NotNil(t, index.full)
		want, err := readSharedStrings(strings.NewReader(data))
		require.NoError(t, err)
		value, err := index.value(0)
		require.NoError(t, err)
		require.Equal(t, want[0], value)
	}
}

func FuzzSharedStringIndexMatchesStrictDecoder(f *testing.F) {
	for _, seed := range []string{`<sst/>`, `<sst><si/><si><t>中文 &amp; _x0041_</t></si></sst>`,
		`<sst><si><r><t>A</t></r><rPh><t>ignored</t></rPh><r><t>B</t></r></si></sst>`,
		`<sst><si><t><![CDATA[A<&B]]></t></si></sst>`, `<sst><si><t><b/></t></si></sst>`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64<<10 {
			t.Skip()
		}
		want, strictErr := readSharedStrings(bytes.NewReader(data))
		index, err := newSharedStringIndex(data)
		require.Equal(t, strictErr == nil, err == nil)
		if err != nil {
			return
		}
		count := len(index.spans)
		if index.full != nil {
			count = len(index.full)
		}
		require.Len(t, want, count)
		for id, value := range want {
			got, err := index.value(id)
			require.NoError(t, err)
			require.Equal(t, value, got)
		}
	})
}

func benchmarkSharedStrings(count int) []byte {
	var data bytes.Buffer
	data.WriteString(`<?xml version="1.0" encoding="UTF-8"?><sst xmlns="urn:sheet">`)
	for id := 0; id < count; id++ {
		fmt.Fprintf(&data, `<si><t>value-%d</t></si>`, id)
	}
	data.WriteString(`</sst>`)
	return data.Bytes()
}

func BenchmarkSharedStringSelection(b *testing.B) {
	data := benchmarkSharedStrings(50_000)
	for _, selected := range []bool{false, true} {
		b.Run(fmt.Sprintf("selective=%t", selected), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if !selected {
					if _, err := readSharedStrings(bytes.NewReader(data)); err != nil {
						b.Fatal(err)
					}
					b.ReportMetric(50_000, "decoded/op")
					continue
				}
				index, err := newSharedStringIndex(data)
				if err != nil {
					b.Fatal(err)
				}
				for id := 0; id < 10; id++ {
					if _, err := index.value(id * 4_999); err != nil {
						b.Fatal(err)
					}
				}
				b.ReportMetric(float64(len(index.values)), "decoded/op")
			}
		})
	}
}
