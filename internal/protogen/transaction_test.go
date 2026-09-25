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

func TestGenerator_commitOutputsRollsBackOnCollision(t *testing.T) {
	dir := t.TempDir()
	gen := newSweepGenerator(t, dir)
	outdir := filepath.Join(dir, "default")
	require.NoError(t, os.MkdirAll(outdir, 0o755))
	require.NoError(t, gen.beginRun())
	require.NoError(t, gen.startStaging())
	defer gen.discardStaging()

	old := filepath.Join(outdir, "a.proto")
	oldContent := []byte(generatedFileHeaderLine() + "old\n")
	require.NoError(t, os.WriteFile(old, oldContent, 0o644))
	require.NoError(t, gen.registerGeneratedProtoFile(old, "A.xlsx"))
	require.NoError(t, gen.stageProtoFile(old, []byte(generatedFileHeaderLine()+"new\n")))

	blocked := filepath.Join(outdir, "z.proto")
	require.NoError(t, gen.registerGeneratedProtoFile(blocked, "Z.xlsx"))
	require.NoError(t, gen.stageProtoFile(blocked, []byte(generatedFileHeaderLine())))
	// Another writer claims the target after registration.
	require.NoError(t, os.Mkdir(blocked, 0o755))

	require.Error(t, gen.commitOutputs(false))
	content, err := os.ReadFile(old)
	require.NoError(t, err)
	require.Equal(t, oldContent, content)
	require.DirExists(t, blocked)
}

func TestGenerator_refusesExistingNonGeneratedOutput(t *testing.T) {
	dir := t.TempDir()
	gen := newSweepGenerator(t, dir)
	outdir := filepath.Join(dir, "default")
	require.NoError(t, os.MkdirAll(outdir, 0o755))
	path := filepath.Join(outdir, "item.proto")
	handwritten := []byte("syntax = \"proto3\";\n")
	require.NoError(t, os.WriteFile(path, handwritten, 0o644))

	require.NoError(t, gen.beginRun())
	err := gen.registerGeneratedProtoFile(path, "Item.xlsx")
	require.ErrorContains(t, err, "non-generated")
	content, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	require.Equal(t, handwritten, content)
}

func TestGenerator_refusesImportedOutputEvenWithGeneratedHeader(t *testing.T) {
	dir := t.TempDir()
	gen := newSweepGenerator(t, dir)
	outdir := filepath.Join(dir, "default")
	require.NoError(t, os.MkdirAll(outdir, 0o755))
	path := filepath.Join(outdir, "shared.proto")
	content := []byte(generatedFileHeaderLine())
	require.NoError(t, os.WriteFile(path, content, 0o644))
	gen.InputOpt.ProtoFiles = []string{path}

	require.NoError(t, gen.beginRun())
	err := gen.registerGeneratedProtoFile(path, "Shared.xlsx")
	require.ErrorContains(t, err, "imported proto")
	require.NoError(t, gen.sweepOutdir())
	got, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	require.Equal(t, content, got)
}

func TestGenerator_commitOutputsRestoresPromotedFile(t *testing.T) {
	dir := t.TempDir()
	gen := newSweepGenerator(t, dir)
	outdir := filepath.Join(dir, "default")
	require.NoError(t, os.MkdirAll(outdir, 0o755))
	require.NoError(t, gen.beginRun())
	require.NoError(t, gen.startStaging())
	defer gen.discardStaging()

	first := filepath.Join(outdir, "a.proto")
	oldContent := []byte(generatedFileHeaderLine() + "old\n")
	require.NoError(t, os.WriteFile(first, oldContent, 0o644))
	require.NoError(t, gen.registerGeneratedProtoFile(first, "A.xlsx"))
	require.NoError(t, gen.stageProtoFile(first, []byte(generatedFileHeaderLine()+"new\n")))

	second := filepath.Join(outdir, "z.proto")
	require.NoError(t, gen.registerGeneratedProtoFile(second, "Z.xlsx"))
	require.NoError(t, gen.stageProtoFile(second, []byte(generatedFileHeaderLine())))
	key, err := absoluteProtoPath(second)
	require.NoError(t, err)
	require.NoError(t, os.Remove(gen.stagedProtoFiles[key]))

	require.Error(t, gen.commitOutputs(false))
	content, err := os.ReadFile(first)
	require.NoError(t, err)
	require.Equal(t, oldContent, content)
	require.NoFileExists(t, second)
}

func TestGenerator_commitOutputsRejectsLateHandwrittenFile(t *testing.T) {
	dir := t.TempDir()
	gen := newSweepGenerator(t, dir)
	outdir := filepath.Join(dir, "default")
	require.NoError(t, os.MkdirAll(outdir, 0o755))
	require.NoError(t, gen.beginRun())
	require.NoError(t, gen.startStaging())
	defer gen.discardStaging()

	path := filepath.Join(outdir, "item.proto")
	require.NoError(t, gen.registerGeneratedProtoFile(path, "Item.xlsx"))
	require.NoError(t, gen.stageProtoFile(path, []byte(generatedFileHeaderLine())))
	handwritten := []byte("syntax = \"proto3\";\n")
	require.NoError(t, os.WriteFile(path, handwritten, 0o644))

	require.ErrorContains(t, gen.commitOutputs(false), "non-generated")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, handwritten, content)
}
