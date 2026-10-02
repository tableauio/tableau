package importer

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/xlsx"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
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

func TestExcelOpenFailuresKeepErrorCode(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"missing", "not ZIP", "truncated ZIP"} {
		t.Run(kind, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "invalid.xlsx")
			switch kind {
			case "not ZIP":
				require.NoError(t, os.WriteFile(filename, []byte("invalid workbook"), 0o600))
			case "truncated ZIP":
				require.NoError(t, os.WriteFile(filename, []byte("PK\x03\x04"), 0o600))
			}
			for _, mode := range []ImporterMode{UnknownMode, Protogen} {
				_, err := NewExcelImporter(ctx, filename, Mode(mode))
				require.ErrorIs(t, err, xerrors.ErrE3002)
			}
			_, err := NewCache().Load(ctx, filename, Mode(Confgen))
			require.ErrorIs(t, err, xerrors.ErrE3002)
		})
	}
}

func TestExcelImportsWorkbookWithChartSheet(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "chart.xlsx")
	file := excelize.NewFile()
	require.NoError(t, file.SetCellValue("Sheet1", "A1", "Name"))
	require.NoError(t, file.SetCellValue("Sheet1", "B1", "Value"))
	require.NoError(t, file.SetCellValue("Sheet1", "A2", "item"))
	require.NoError(t, file.SetCellValue("Sheet1", "B2", 1))
	_, err := file.NewSheet("@TABLEAU")
	require.NoError(t, err)
	require.NoError(t, file.SetCellValue("@TABLEAU", "A1", "Sheet"))
	require.NoError(t, file.AddChartSheet("Chart", &excelize.Chart{
		Type: excelize.Col,
		Series: []excelize.ChartSeries{{
			Name: "Sheet1!$B$1", Categories: "Sheet1!$A$2", Values: "Sheet1!$B$2",
		}},
	}))
	require.NoError(t, file.SaveAs(filename))
	require.NoError(t, file.Close())

	_, err = xlsx.Open(filename)
	require.ErrorIs(t, err, xlsx.ErrUnsupported)
	want := [][]string{{"Name", "Value"}, {"item", "1"}}
	imp, err := NewExcelImporter(context.Background(), filename,
		Mode(Protogen), Sheets([]string{"@TABLEAU", "Sheet1"}))
	require.NoError(t, err)
	require.Equal(t, want, imp.GetSheet("Sheet1").Table.Rows)

	cached, err := openCachedExcel(filename, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cached.reader.Close()) })
	require.Equal(t, "excelize", cached.readerName())
	view, _, err := cached.load(context.Background(), []string{"Sheet1"})
	require.NoError(t, err)
	require.Equal(t, want, view.GetSheet("Sheet1").Table.Rows)

	view, err = NewCache().Load(context.Background(), filename,
		Mode(Confgen), Sheets([]string{"Sheet1"}))
	require.NoError(t, err)
	require.Equal(t, want, view.GetSheet("Sheet1").Table.Rows)
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

func TestExcelImportsUTF16Worksheet(t *testing.T) {
	for _, order := range []unicode.Endianness{unicode.LittleEndian, unicode.BigEndian} {
		for _, bom := range []unicode.BOMPolicy{unicode.UseBOM, unicode.IgnoreBOM} {
			t.Run(fmt.Sprintf("order=%v/bom=%v", order, bom), func(t *testing.T) {
				worksheet := `<?xml version="1.0" encoding="UTF-16"?><worksheet><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>中文 café</t></is></c></row></sheetData></worksheet>`
				encoded, _, err := transform.Bytes(unicode.UTF16(order, bom).NewEncoder(), []byte(worksheet))
				require.NoError(t, err)
				filename := writeXLSXWithWorksheet(t, string(encoded))
				reader, err := xlsx.Open(filename)
				require.NoError(t, err)
				want := [][]string{{"中文 café"}}
				for _, limit := range []uint{0, 1} {
					rows, err := reader.ReadRowsN("Sheet1", limit)
					require.NoError(t, err)
					require.Equal(t, want, rows)
				}
				require.NoError(t, reader.Close())
				imp, err := NewExcelImporter(context.Background(), filename, Mode(Protogen))
				require.NoError(t, err)
				require.Equal(t, want, imp.GetSheet("Sheet1").Table.Rows)
				cached, err := openCachedExcel(filename, false)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, cached.reader.Close()) })
				view, _, err := cached.load(context.Background(), []string{"Sheet1"})
				require.NoError(t, err)
				require.Equal(t, want, view.GetSheet("Sheet1").Table.Rows)
			})
		}
	}
}

func TestExcelImportsFallbackForLegacyCharset(t *testing.T) {
	filename := writeXLSXWithWorksheet(t, "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><worksheet><sheetData><row r=\"1\"><c r=\"A1\" t=\"inlineStr\"><is><t>caf\xe9</t></is></c></row></sheetData></worksheet>")
	reader, err := xlsx.Open(filename)
	require.NoError(t, err)
	for _, limit := range []uint{0, 1} {
		_, err := reader.ReadRowsN("Sheet1", limit)
		require.ErrorIs(t, err, xlsx.ErrUnsupported)
	}
	require.NoError(t, reader.Close())
	want := [][]string{{"café"}}
	imp, err := NewExcelImporter(context.Background(), filename, Mode(Protogen))
	require.NoError(t, err)
	require.Equal(t, want, imp.GetSheet("Sheet1").Table.Rows)
	cached, err := openCachedExcel(filename, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cached.reader.Close()) })
	view, _, err := cached.load(context.Background(), []string{"Sheet1"})
	require.NoError(t, err)
	require.Equal(t, want, view.GetSheet("Sheet1").Table.Rows)
	require.Equal(t, "excelize", cached.readerName())
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
	_, err := file.NewSheet("@TABLEAU")
	require.NoError(t, err)
	require.NoError(t, file.SetCellValue("@TABLEAU", "A1", "Sheet"))
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
