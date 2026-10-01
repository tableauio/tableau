package importer

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/xlsx"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"github.com/xuri/excelize/v2"
	"google.golang.org/protobuf/proto"
)

type excelRead struct {
	sheetName string
	topN      uint
}

type recordingExcelReader struct {
	sheetNames []string
	rows       map[string][][]string
	reads      []excelRead
}

func (r *recordingExcelReader) SheetNames() []string {
	return append([]string(nil), r.sheetNames...)
}

func (r *recordingExcelReader) ReadRows(sheetName string, topN uint) ([][]string, error) {
	r.reads = append(r.reads, excelRead{sheetName: sheetName, topN: topN})
	rows := r.rows[sheetName]
	if topN > 0 && uint(len(rows)) > topN {
		rows = rows[:topN]
	}
	return rows, nil
}

func (r *recordingExcelReader) Close() error { return nil }

type fixedExcelMetasheetParser struct {
	meta *internalpb.Metabook
}

func (p fixedExcelMetasheetParser) Parse(message proto.Message, _ *book.Sheet) error {
	proto.Merge(message, p.meta)
	return nil
}

func TestReadExcelImporterUsesMetasheetReadPlan(t *testing.T) {
	reader := &recordingExcelReader{
		sheetNames: []string{"@TABLEAU", "Keep", "Drop", "Transpose"},
		rows: map[string][][]string{
			"@TABLEAU":  {{"Sheet"}, {"metadata"}},
			"Keep":      numberedRows(12),
			"Drop":      numberedRows(12),
			"Transpose": numberedRows(12),
		},
	}
	parser := fixedExcelMetasheetParser{meta: &internalpb.Metabook{
		MetasheetMap: map[string]*internalpb.Metasheet{
			"Keep":      {Sheet: "Keep"},
			"Transpose": {Sheet: "Transpose", Transpose: true},
		},
	}}

	imp, err := readExcelImporter(context.Background(), "Book.xlsx", &Options{
		Mode:   Protogen,
		Parser: parser,
	}, reader, true)
	require.NoError(t, err)
	require.Equal(t, []excelRead{
		{sheetName: "@TABLEAU"},
		{sheetName: "Keep", topN: defaultTopN},
		{sheetName: "Transpose"},
	}, reader.reads)
	require.Equal(t, []string{"Keep", "Transpose"}, sheetNames(imp.GetSheets()))
	require.Equal(t, int(defaultTopN), imp.GetSheet("Keep").Table.RowSize())
	require.Equal(t, 12, imp.GetSheet("Transpose").Table.RowSize())
}

func TestNewExcelImporterReadsCDATA(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.xlsx")
	file := excelize.NewFile()
	require.NoError(t, file.SetCellValue("Sheet1", "A1", "value"))
	require.NoError(t, file.SaveAs(source))
	require.NoError(t, file.Close())

	filename := filepath.Join(t.TempDir(), "fallback.xlsx")
	copyXLSXWithCDATA(t, source, filename)

	reader, err := xlsx.Open(filename)
	require.NoError(t, err)
	rows, err := reader.ReadRows("Sheet1")
	require.NoError(t, err)
	require.Equal(t, [][]string{{"value"}}, rows)
	require.NoError(t, reader.Close())

	compat, err := excelize.OpenFile(filename)
	require.NoError(t, err)
	rows, err = compat.GetRows("Sheet1", excelize.Options{RawCellValue: true})
	require.NoError(t, err)
	require.Equal(t, [][]string{{"value"}}, rows)
	require.NoError(t, compat.Close())

	_, err = NewExcelImporter(context.Background(), filename, Mode(Protogen))
	require.NoError(t, err)
}

func TestNewExcelImporterFallsBackForDocumentType(t *testing.T) {
	filename := writeXLSXWithWorksheet(t, `<!DOCTYPE worksheet><worksheet><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>value</t></is></c></row></sheetData></worksheet>`)
	reader, err := xlsx.Open(filename)
	require.NoError(t, err)
	_, err = reader.ReadRows("Sheet1")
	require.ErrorIs(t, err, xlsx.ErrUnsupported)
	require.NoError(t, reader.Close())

	_, err = NewExcelImporter(context.Background(), filename, Mode(Protogen))
	require.NoError(t, err)
}

func TestNewExcelImporterPreservesNamedEntity(t *testing.T) {
	filename := writeXLSXWithWorksheet(t, `<worksheet><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>&nbsp;</t></is></c></row></sheetData></worksheet>`)
	reader, err := xlsx.Open(filename)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reader.Close()) })
	rows, err := reader.ReadRows("Sheet1")
	require.NoError(t, err)
	require.Equal(t, [][]string{{"\u00a0"}}, rows)
	_, err = NewExcelImporter(context.Background(), filename, Mode(Protogen))
	require.NoError(t, err)
}

func TestProtogenDoesNotRetryInvalidEntity(t *testing.T) {
	filename := writeXLSXWithWorksheet(t, `<worksheet><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>&xyzzytableau;</t></is></c></row></sheetData></worksheet>`)
	_, err := NewExcelImporter(context.Background(), filename, Mode(Protogen))
	require.ErrorContains(t, err, "invalid character entity")
}

func numberedRows(count int) [][]string {
	rows := make([][]string, count)
	for i := range rows {
		rows[i] = []string{"row"}
	}
	return rows
}

func sheetNames(sheets []*book.Sheet) []string {
	names := make([]string, len(sheets))
	for i, sheet := range sheets {
		names[i] = sheet.Name
	}
	return names
}

func copyXLSXWithCDATA(t *testing.T, source, target string) {
	rewriteXLSXWorksheet(t, source, target, func(data []byte) []byte {
		updated := bytes.Replace(data, []byte("<sheetData>"), []byte("<![CDATA[ignored]]><sheetData>"), 1)
		require.NotEqual(t, data, updated)
		return updated
	})
}

func writeXLSXWithWorksheet(t *testing.T, worksheet string) string {
	t.Helper()
	source := filepath.Join(t.TempDir(), "source.xlsx")
	file := excelize.NewFile()
	require.NoError(t, file.SaveAs(source))
	require.NoError(t, file.Close())
	target := filepath.Join(t.TempDir(), "worksheet.xlsx")
	rewriteXLSXWorksheet(t, source, target, func([]byte) []byte {
		return []byte(worksheet)
	})
	return target
}

func rewriteXLSXWorksheet(t *testing.T, source, target string, rewrite func([]byte) []byte) {
	t.Helper()
	input, err := zip.OpenReader(source)
	require.NoError(t, err)
	defer func() { require.NoError(t, input.Close()) }()

	output, err := os.Create(target)
	require.NoError(t, err)
	archive := zip.NewWriter(output)
	foundWorksheet := false
	for _, entry := range input.File {
		reader, err := entry.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.NoError(t, reader.Close())

		if entry.Name == "xl/worksheets/sheet1.xml" {
			data = rewrite(data)
			foundWorksheet = true
		}
		writer, err := archive.CreateHeader(&entry.FileHeader)
		require.NoError(t, err)
		_, err = writer.Write(data)
		require.NoError(t, err)
	}
	require.True(t, foundWorksheet)
	require.NoError(t, archive.Close())
	require.NoError(t, output.Close())
}
