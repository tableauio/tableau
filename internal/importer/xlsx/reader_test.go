package xlsx

import (
	"archive/zip"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestReaderMatchesExcelize(t *testing.T) {
	var filenames []string
	for _, root := range []string{filepath.Join("..", "testdata"), filepath.Join("..", "..", "..", "test")} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ".xlsx") {
				filenames = append(filenames, path)
			}
			return nil
		})
		require.NoError(t, err)
	}
	sort.Strings(filenames)
	require.NotEmpty(t, filenames)

	for _, filename := range filenames {
		filename := filename
		t.Run(filepath.ToSlash(filename), func(t *testing.T) {
			reader, err := Open(filename)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, reader.Close()) })

			file, err := excelize.OpenFile(filename)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, file.Close()) })

			require.Equal(t, file.GetSheetList(), reader.SheetNames())
			for _, sheetName := range reader.SheetNames() {
				want, err := file.GetRows(sheetName, excelize.Options{RawCellValue: true})
				require.NoError(t, err, "read Excelize sheet %q", sheetName)
				got, err := reader.ReadRows(sheetName)
				require.NoError(t, err, "read XLSX sheet %q", sheetName)
				require.Equal(t, want, got, "sheet %q", sheetName)
			}
		})
	}
}

func TestReaderUsesPackageRelationships(t *testing.T) {
	filename := writeArchive(t, map[string]string{
		"_rels/.rels":     `<Relationships><Relationship Id="root" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="/custom/book.xml"/></Relationships>`,
		"custom/book.xml": `<workbook xmlns:r="urn:relationships"><sheets><sheet name="Items" r:id="sheet"/></sheets></workbook>`,
		"custom/_rels/book.xml.rels": `<Relationships>
<Relationship Id="sheet" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/items.xml"/>
<Relationship Id="strings" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="strings.xml"/>
</Relationships>`,
		"custom/worksheets/items.xml": `<worksheet><sheetData><row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row></sheetData></worksheet>`,
		"custom/strings.xml":          `<sst><si><t>A_x0042_</t></si><si><r><t>Hello </t></r><r><t>world</t></r></si></sst>`,
	})

	reader, err := Open(filename)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reader.Close()) })

	names := reader.SheetNames()
	require.Equal(t, []string{"Items"}, names)
	names[0] = "changed"
	require.Equal(t, []string{"Items"}, reader.SheetNames())
	rows, err := reader.ReadRows("Items")
	require.NoError(t, err)
	require.Equal(t, [][]string{{"AB", "Hello world"}}, rows)
	_, err = reader.ReadRows("Missing")
	require.ErrorIs(t, err, ErrSheetNotFound)
}

func TestOpenRejectsInvalidWorkbooks(t *testing.T) {
	validWorkbook := `<workbook xmlns:r="urn:relationships"><sheets><sheet name="Items" r:id="sheet"/></sheets></workbook>`
	validRels := `<Relationships><Relationship Id="sheet" Type="worksheet" Target="worksheets/items.xml"/></Relationships>`
	duplicateShared := defaultParts(validWorkbook, `<Relationships><Relationship Id="sheet" Type="worksheet" Target="worksheets/items.xml"/><Relationship Id="strings1" Type="sharedStrings" Target="one.xml"/><Relationship Id="strings2" Type="sharedStrings" Target="two.xml"/></Relationships>`, `<worksheet/>`)
	duplicateShared["xl/one.xml"] = `<sst/>`
	duplicateShared["xl/two.xml"] = `<sst/>`
	tests := []struct {
		name    string
		parts   map[string]string
		wantErr string
	}{
		{name: "missing workbook", parts: map[string]string{}, wantErr: `XLSX entry "xl/workbook.xml" not found`},
		{name: "malformed root relationships", parts: map[string]string{"_rels/.rels": `<Relationships`}, wantErr: "decode XLSX entry"},
		{name: "missing workbook relationships", parts: map[string]string{"xl/workbook.xml": validWorkbook}, wantErr: `XLSX entry "xl/_rels/workbook.xml.rels" not found`},
		{name: "missing worksheet", parts: map[string]string{"xl/workbook.xml": validWorkbook, "xl/_rels/workbook.xml.rels": validRels}, wantErr: `relationship "sheet" not found`},
		{name: "wrong relationship type", parts: defaultParts(validWorkbook, `<Relationships><Relationship Id="sheet" Type="styles" Target="worksheets/items.xml"/></Relationships>`, `<worksheet/>`), wantErr: `relationship "sheet" not found`},
		{name: "duplicate relationship", parts: defaultParts(validWorkbook, `<Relationships><Relationship Id="sheet" Type="worksheet" Target="worksheets/items.xml"/><Relationship Id="sheet" Type="styles" Target="styles.xml"/></Relationships>`, `<worksheet/>`), wantErr: "duplicate workbook relationship"},
		{name: "empty relationship ID", parts: defaultParts(validWorkbook, `<Relationships><Relationship Id="" Type="worksheet" Target="worksheets/items.xml"/></Relationships>`, `<worksheet/>`), wantErr: "empty ID"},
		{name: "missing shared strings", parts: defaultParts(validWorkbook, `<Relationships><Relationship Id="sheet" Type="worksheet" Target="worksheets/items.xml"/><Relationship Id="strings" Type="sharedStrings" Target="missing.xml"/></Relationships>`, `<worksheet/>`), wantErr: "shared strings relationship"},
		{name: "duplicate shared strings", parts: duplicateShared, wantErr: "duplicate shared strings relationship"},
		{name: "empty sheet name", parts: defaultParts(`<workbook xmlns:r="urn:relationships"><sheets><sheet name="" r:id="sheet"/></sheets></workbook>`, validRels, `<worksheet/>`), wantErr: "empty name"},
		{name: "external worksheet", parts: defaultParts(validWorkbook, `<Relationships><Relationship Id="sheet" Type="worksheet" Target="https://example.com/items.xml" TargetMode="External"/></Relationships>`, `<worksheet/>`), wantErr: `relationship "sheet" not found`},
		{name: "duplicate sheet", parts: map[string]string{
			"xl/workbook.xml":            `<workbook xmlns:r="urn:relationships"><sheets><sheet name="Items" r:id="one"/><sheet name="Items" r:id="two"/></sheets></workbook>`,
			"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="one" Type="worksheet" Target="worksheets/one.xml"/><Relationship Id="two" Type="worksheet" Target="worksheets/two.xml"/></Relationships>`,
			"xl/worksheets/one.xml":      `<worksheet/>`,
			"xl/worksheets/two.xml":      `<worksheet/>`,
		}, wantErr: "duplicate worksheet"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Open(writeArchive(t, test.parts))
			require.ErrorContains(t, err, test.wantErr)
		})
	}

	filename := filepath.Join(t.TempDir(), "invalid.xlsx")
	require.NoError(t, os.WriteFile(filename, []byte("not a zip"), 0o600))
	_, err := Open(filename)
	require.Error(t, err)
}

func TestOpenRejectsAmbiguousPartNames(t *testing.T) {
	tests := []struct {
		name    string
		parts   []archivePart
		wantErr string
	}{
		{
			name: "duplicate",
			parts: []archivePart{
				{name: "xl/workbook.xml", data: "first"},
				{name: "XL/WORKBOOK.XML", data: "second"},
			},
			wantErr: "duplicate XLSX entry",
		},
		{
			name:    "parent path",
			parts:   []archivePart{{name: "../workbook.xml", data: "invalid"}},
			wantErr: "invalid XLSX entry path",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Open(writeArchiveParts(t, test.parts))
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestReadRowsRejectsUnsupportedXML(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{name: "CDATA", xml: `<worksheet><sheetData><row><c><v><![CDATA[value]]></v></c></row></sheetData></worksheet>`},
		{name: "DOCTYPE", xml: `<!DOCTYPE worksheet><worksheet/>`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader, err := Open(writeArchive(t, defaultParts(
				`<workbook xmlns:r="urn:relationships"><sheets><sheet name="Items" r:id="sheet"/></sheets></workbook>`,
				`<Relationships><Relationship Id="sheet" Type="worksheet" Target="worksheets/items.xml"/></Relationships>`,
				test.xml,
			)))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, reader.Close()) })
			_, err = reader.ReadRows("Items")
			require.ErrorContains(t, err, "unsupported XML construct")
		})
	}
}

func TestReadRowsRejectsMalformedSharedStrings(t *testing.T) {
	parts := defaultParts(
		`<workbook xmlns:r="urn:relationships"><sheets><sheet name="Items" r:id="sheet"/></sheets></workbook>`,
		`<Relationships><Relationship Id="sheet" Type="worksheet" Target="worksheets/items.xml"/><Relationship Id="strings" Type="sharedStrings" Target="sharedStrings.xml"/></Relationships>`,
		`<worksheet><sheetData><row><c t="s"><v>0</v></c></row></sheetData></worksheet>`,
	)
	parts["xl/sharedStrings.xml"] = `<sst><si>`
	reader, err := Open(writeArchive(t, parts))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reader.Close()) })

	_, err = reader.ReadRows("Items")
	require.ErrorContains(t, err, "decode XLSX shared strings")
	_, secondErr := reader.ReadRows("Items")
	require.EqualError(t, secondErr, err.Error())
}

func TestPartPaths(t *testing.T) {
	require.Equal(t, "xl/workbook.xml", normalizePartName(`/XL\WORKBOOK.XML`))
	require.Equal(t, "custom/sheets/items.xml", resolvePartPath("custom/book.xml", `sheets\items.xml`))
	require.Equal(t, "xl/workbook.xml", resolvePartPath("", "/xl/workbook.xml"))
}

func TestReadEntryRejectsOversizedPart(t *testing.T) {
	_, err := readEntry(&zip.File{FileHeader: zip.FileHeader{
		Name:               "large.xml",
		UncompressedSize64: maxEntrySize + 1,
	}})
	require.ErrorContains(t, err, "exceeds")
}

func TestNilReaderClose(t *testing.T) {
	var reader *Reader
	require.NoError(t, reader.Close())
}

func defaultParts(workbook, rels, sheet string) map[string]string {
	return map[string]string{
		"xl/workbook.xml":            workbook,
		"xl/_rels/workbook.xml.rels": rels,
		"xl/worksheets/items.xml":    sheet,
	}
}

func writeArchive(t *testing.T, parts map[string]string) string {
	t.Helper()
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]archivePart, 0, len(names))
	for _, name := range names {
		entries = append(entries, archivePart{name: name, data: parts[name]})
	}
	return writeArchiveParts(t, entries)
}

type archivePart struct {
	name string
	data string
}

func writeArchiveParts(t *testing.T, parts []archivePart) string {
	t.Helper()
	filename := filepath.Join(t.TempDir(), "test.xlsx")
	file, err := os.Create(filename)
	require.NoError(t, err)
	archive := zip.NewWriter(file)
	for _, entry := range parts {
		part, err := archive.Create(entry.name)
		require.NoError(t, err)
		_, err = part.Write([]byte(entry.data))
		require.NoError(t, err)
	}
	require.NoError(t, archive.Close())
	require.NoError(t, file.Close())
	return filename
}
