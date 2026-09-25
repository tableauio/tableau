package protogen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/options"
)

func TestGenerator_reuseAndFailedRunPreservesOutputs(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()
	workbook := filepath.Join(inputDir, "Items.yaml")
	valid := []byte(`"@sheet": "@TABLEAU"
---
"@sheet": "@ItemConf"
ID: uint32
Name: string
`)
	require.NoError(t, os.WriteFile(workbook, valid, 0o644))

	gen := NewGenerator("testconf", inputDir, outputDir,
		options.Proto(&options.ProtoOption{
			Input:  &options.ProtoInputOption{Formats: []format.Format{format.YAML}},
			Output: &options.ProtoOutputOption{},
		}))
	require.NoError(t, gen.Generate())
	generated := filepath.Join(outputDir, "items.proto")
	before, err := os.ReadFile(generated)
	require.NoError(t, err)

	// Run state must be reset when the same Generator is used again.
	require.NoError(t, gen.Generate())
	again, err := os.ReadFile(generated)
	require.NoError(t, err)
	require.Equal(t, before, again)

	stale := filepath.Join(outputDir, "old.proto")
	require.NoError(t, os.WriteFile(stale, []byte(generatedFileHeaderLine()), 0o644))
	updated := []byte(`"@sheet": "@TABLEAU"
---
"@sheet": "@ItemConf"
ID: uint32
Name: string
Level: int32
`)
	require.NoError(t, os.WriteFile(workbook, updated, 0o644))
	broken := filepath.Join(inputDir, "Broken.yaml")
	require.NoError(t, os.WriteFile(broken, []byte(`"@sheet": "@TABLEAU"
---
"@sheet": "@BrokenConf"
Bad: .MissingType
`), 0o644))
	require.Error(t, gen.Generate())
	after, err := os.ReadFile(generated)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.FileExists(t, stale)

	require.NoError(t, os.Remove(broken))
	require.NoError(t, gen.Generate())
	require.NoFileExists(t, stale)
	committed, err := os.ReadFile(generated)
	require.NoError(t, err)
	require.NotEqual(t, before, committed)

}

func TestProtoOutput_publishStopsBeforeStaleCleanup(t *testing.T) {
	dir := t.TempDir()
	gen := newSweepGenerator(t, dir)
	outdir := filepath.Join(dir, "default")
	require.NoError(t, os.MkdirAll(outdir, 0o755))
	require.NoError(t, gen.resetRunState())
	require.NoError(t, gen.output.createStagingDir())
	defer gen.output.removeStagingDir()

	first := filepath.Join(outdir, "a.proto")
	require.NoError(t, os.WriteFile(first, []byte(generatedFileHeaderLine()+"old\\n"), 0o644))
	require.NoError(t, gen.output.reservePath(first, "A.xlsx"))
	require.NoError(t, gen.output.stageFile(first, []byte(generatedFileHeaderLine()+"new\\n")))

	blocked := filepath.Join(outdir, "z.proto")
	require.NoError(t, gen.output.reservePath(blocked, "Z.xlsx"))
	require.NoError(t, gen.output.stageFile(blocked, []byte(generatedFileHeaderLine())))
	require.NoError(t, os.Mkdir(blocked, 0o755))

	stale := filepath.Join(outdir, "stale.proto")
	require.NoError(t, os.WriteFile(stale, []byte(generatedFileHeaderLine()), 0o644))
	require.Error(t, gen.output.publishAll())
	content, err := os.ReadFile(first)
	require.NoError(t, err)
	require.Equal(t, generatedFileHeaderLine()+"new\\n", string(content))
	require.FileExists(t, stale)
	require.DirExists(t, blocked)
}

func TestProtoOutput_reservePathRejectsNonGenerated(t *testing.T) {
	dir := t.TempDir()
	gen := newSweepGenerator(t, dir)
	outdir := filepath.Join(dir, "default")
	require.NoError(t, os.MkdirAll(outdir, 0o755))
	path := filepath.Join(outdir, "item.proto")
	handwritten := []byte("syntax = \"proto3\";\n")
	require.NoError(t, os.WriteFile(path, handwritten, 0o644))

	require.NoError(t, gen.resetRunState())
	err := gen.output.reservePath(path, "Item.xlsx")
	require.ErrorContains(t, err, "non-generated")
	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	require.Equal(t, handwritten, content)
}

func TestProtoOutput_reservePathRejectsImported(t *testing.T) {
	dir := t.TempDir()
	gen := newSweepGenerator(t, dir)
	outdir := filepath.Join(dir, "default")
	require.NoError(t, os.MkdirAll(outdir, 0o755))
	path := filepath.Join(outdir, "shared.proto")
	content := []byte(generatedFileHeaderLine())
	require.NoError(t, os.WriteFile(path, content, 0o644))
	gen.InputOpt.ProtoFiles = []string{path}

	require.NoError(t, gen.resetRunState())
	err := gen.output.reservePath(path, "Shared.xlsx")
	require.ErrorContains(t, err, "imported proto")
	require.NoError(t, gen.output.createStagingDir())
	defer gen.output.removeStagingDir()
	require.NoError(t, gen.output.publishAll())
	got, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	require.Equal(t, content, got)
}

func TestProtoOutput_publishReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	gen := newSweepGenerator(t, dir)
	outdir := filepath.Join(dir, "default")
	require.NoError(t, os.MkdirAll(outdir, 0o755))
	require.NoError(t, gen.resetRunState())
	require.NoError(t, gen.output.createStagingDir())
	defer gen.output.removeStagingDir()

	path := filepath.Join(outdir, "item.proto")
	require.NoError(t, os.WriteFile(path, []byte(generatedFileHeaderLine()+"old\\n"), 0o644))
	require.NoError(t, gen.output.reservePath(path, "Item.xlsx"))
	replacement := generatedFileHeaderLine() + "new\\n"
	require.NoError(t, gen.output.stageFile(path, []byte(replacement)))

	require.NoError(t, gen.output.publishSelected())
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, replacement, string(content))
}

func TestProtoOutput_publishRejectsLateHandwrittenFile(t *testing.T) {
	dir := t.TempDir()
	gen := newSweepGenerator(t, dir)
	outdir := filepath.Join(dir, "default")
	require.NoError(t, os.MkdirAll(outdir, 0o755))
	require.NoError(t, gen.resetRunState())
	require.NoError(t, gen.output.createStagingDir())
	defer gen.output.removeStagingDir()

	path := filepath.Join(outdir, "item.proto")
	require.NoError(t, gen.output.reservePath(path, "Item.xlsx"))
	require.NoError(t, gen.output.stageFile(path, []byte(generatedFileHeaderLine())))
	handwritten := []byte("syntax = \"proto3\";\n")
	require.NoError(t, os.WriteFile(path, handwritten, 0o644))

	require.ErrorContains(t, gen.output.publishSelected(), "non-generated")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, handwritten, content)
}

func TestProtoOutput_stageFileRequiresReservation(t *testing.T) {
	dir := t.TempDir()
	gen := newSweepGenerator(t, dir)
	outdir := filepath.Join(dir, "default")
	require.NoError(t, os.MkdirAll(outdir, 0o755))
	require.NoError(t, gen.resetRunState())
	require.NoError(t, gen.output.createStagingDir())
	defer gen.output.removeStagingDir()

	err := gen.output.stageFile(filepath.Join(outdir, "item.proto"), []byte(generatedFileHeaderLine()))
	require.ErrorContains(t, err, "was not reserved")
}
