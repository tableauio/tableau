package protogen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/options"
)

func TestGenAll_SkipsCSVFileWithoutBookSheetName(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()
	writeFile(t, filepath.Join(inputDir, "Item#@TABLEAU.csv"), "Sheet\nItemConf\n")
	writeFile(t, filepath.Join(inputDir, "Item#ItemConf.csv"), ""+
		"ID,Name\n"+
		"\"map<uint32, Item>\",string\n"+
		"ID,名称\n"+
		"1,apple\n")
	// A CSV that is not a tableau sheet. Protogen directory scans must ignore
	// it, the same way an Excel workbook without @TABLEAU is ignored.
	writeFile(t, filepath.Join(inputDir, "NotAWorkbook.csv"), "a,b\n1,2\n")
	require.NoError(t, os.MkdirAll(filepath.Join(inputDir, "extra"), 0o755))
	writeFile(t, filepath.Join(inputDir, "extra", "AlsoNotAWorkbook.csv"), "x\n")

	gen := newGenAllTestGenerator(inputDir, outputDir, format.CSV)
	require.NoError(t, gen.Generate())

	proto := readFile(t, filepath.Join(outputDir, "item.proto"))
	require.Contains(t, proto, "message ItemConf")
	require.NoFileExists(t, filepath.Join(outputDir, "not_a_workbook.proto"))
}

func TestGenAll_OnlyIrregularCSVSucceeds(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()
	writeFile(t, filepath.Join(inputDir, "NotAWorkbook.csv"), "a,b\n1,2\n")

	gen := newGenAllTestGenerator(inputDir, outputDir, format.CSV)
	require.NoError(t, gen.Generate())
	require.NoFileExists(t, filepath.Join(outputDir, "not_a_workbook.proto"))
}

func TestGenerate_ExplicitCSVWithoutBookSheetNameStillFails(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()
	writeFile(t, filepath.Join(inputDir, "NotAWorkbook.csv"), "a,b\n1,2\n")

	gen := newGenAllTestGenerator(inputDir, outputDir, format.CSV)
	err := gen.Generate("NotAWorkbook.csv")
	require.Error(t, err)
	require.Contains(t, err.Error(), "NotAWorkbook.csv")
	require.Contains(t, err.Error(), "cannot parse the book name")
}

func TestGenAll_SkipsYAMLWithoutMetasheet(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()
	writeFile(t, filepath.Join(inputDir, "Item.yaml"), ""+
		"\"@sheet\": \"@TABLEAU\"\n"+
		"---\n"+
		"\"@sheet\": \"@ItemConf\"\n"+
		"ID: uint32\n"+
		"Name: string\n"+
		"---\n"+
		"\"@sheet\": ItemConf\n"+
		"ID: 1\n"+
		"Name: apple\n")
	writeFile(t, filepath.Join(inputDir, "NotAWorkbook.yaml"), "name: notes\nitems:\n  - 1\n")

	gen := newGenAllTestGenerator(inputDir, outputDir, format.YAML)
	require.NoError(t, gen.Generate())

	proto := readFile(t, filepath.Join(outputDir, "item.proto"))
	require.Contains(t, proto, "message ItemConf")
	require.NoFileExists(t, filepath.Join(outputDir, "not_a_workbook.proto"))
}

func TestGenAll_SkipsXMLWithoutMetasheet(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()
	writeFile(t, filepath.Join(inputDir, "Item.xml"), ""+
		"<?xml version=\"1.0\" encoding=\"UTF-8\" ?>\n"+
		"<!--\n"+
		"<@TABLEAU>\n"+
		"    <Item Sheet=\"ItemConf\" />\n"+
		"</@TABLEAU>\n"+
		"\n"+
		"<ItemConf>\n"+
		"    <ID>uint32</ID>\n"+
		"    <Name>string</Name>\n"+
		"</ItemConf>\n"+
		"-->\n"+
		"<ItemConf>\n"+
		"    <ID>1</ID>\n"+
		"    <Name>apple</Name>\n"+
		"</ItemConf>\n")
	writeFile(t, filepath.Join(inputDir, "NotAWorkbook.xml"), "<note>hello</note>\n")

	gen := newGenAllTestGenerator(inputDir, outputDir, format.XML)
	require.NoError(t, gen.Generate())

	proto := readFile(t, filepath.Join(outputDir, "item.proto"))
	require.Contains(t, proto, "message ItemConf")
	require.NoFileExists(t, filepath.Join(outputDir, "not_a_workbook.proto"))
}

func newGenAllTestGenerator(inputDir, outputDir string, f format.Format) *Generator {
	return NewGenerator("protoconf", inputDir, outputDir,
		options.Proto(
			&options.ProtoOption{
				Input: &options.ProtoInputOption{
					Formats: []format.Format{f},
				},
				Output: &options.ProtoOutputOption{},
			},
		),
	)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}
