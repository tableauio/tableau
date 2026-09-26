package importer

import (
	"archive/zip"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestXLSXReaderMatchesExcelize(t *testing.T) {
	var filenames []string
	for _, root := range []string{"testdata", filepath.Join("..", "..", "test")} {
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
			reader, err := openXLSXReader(filename)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, reader.Close()) })

			file, err := excelize.OpenFile(filename)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, file.Close()) })

			require.Equal(t, file.GetSheetList(), reader.SheetNames())
			for _, sheetName := range reader.SheetNames() {
				want, err := readExcelSheetRows(file, sheetName, 0, excelize.Options{RawCellValue: true})
				require.NoError(t, err, "read Excelize sheet %q", sheetName)
				got, err := reader.ReadRows(sheetName)
				require.NoError(t, err, "read XLSX sheet %q", sheetName)
				require.Equal(t, want, got, "sheet %q", sheetName)
			}
		})
	}
}

func TestParseXLSXRows(t *testing.T) {
	data := []byte(`<worksheet><sheetData>
<row r="2"><c r="B2" t="s"><v>0</v></c><c r="C2" t="inlineStr"><is><t>A&amp;B</t></is></c><c r="D2"><f>1+1</f><v></v></c></row>
<row r="4"><c r="A4"><v>42</v></c></row>
</sheetData></worksheet>`)

	rows, err := parseXLSXRows(data, []string{"shared"})
	require.NoError(t, err)
	require.Equal(t, [][]string{
		nil,
		{"", "shared", "A&B", ""},
		nil,
		{"42"},
	}, rows)
}

func TestCachedExcelFallsBackToExcelize(t *testing.T) {
	cached, err := openCachedExcel("testdata/Test.xlsx", true)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cached.close()) })

	forcedFallback := false
	for _, sheet := range cached.reader.sheets {
		if sheet.name == "Item" {
			// Simulate a worksheet that exceeds the direct reader's limit. The
			// general Excelize reader remains the compatibility path.
			sheet.entry.UncompressedSize64 = maxXLSXEntrySize + 1
			forcedFallback = true
			break
		}
	}
	require.True(t, forcedFallback)
	rows, err := cached.readRows("Item")
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.Nil(t, cached.reader)
	require.NotNil(t, cached.file)

	file, err := cached.openExcelize()
	require.NoError(t, err)
	require.Same(t, cached.file, file)
	rows, err = cached.readRows("Item")
	require.NoError(t, err)
	require.NotEmpty(t, rows)
}

func TestXLSXReaderRejectsMissingAndOversizedEntries(t *testing.T) {
	reader, err := openXLSXReader("testdata/Test.xlsx")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reader.Close()) })

	_, err = reader.ReadRows("Missing")
	require.ErrorIs(t, err, ErrSheetNotFound)
	_, err = readZipEntry(&zip.File{FileHeader: zip.FileHeader{
		Name:               "large.xml",
		UncompressedSize64: maxXLSXEntrySize + 1,
	}})
	require.ErrorContains(t, err, "exceeds")
}

func TestDecodeExcelEscapes(t *testing.T) {
	require.Equal(t, "A", decodeExcelEscapes("_x0041_"))
	require.Equal(t, "_x0041_", decodeExcelEscapes("_x005F_x0041_"))
	require.Equal(t, "plain", decodeExcelEscapes("plain"))
}
