package protogen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/options"
)

func newSweepGenerator(t *testing.T, outputDir string) *Generator {
	t.Helper()
	return NewGeneratorWithOptions("protoconf", "testdata", outputDir, &options.Options{
		LocationName: "Asia/Shanghai",
		Proto: &options.ProtoOption{
			Input:  &options.ProtoInputOption{ProtoPaths: []string{"../../proto"}},
			Output: &options.ProtoOutputOption{Subdir: "default"},
		},
	})
}

func TestProtoOutput_publishKeepsHandwrittenProto(t *testing.T) {
	outputDir := t.TempDir()
	gen := newSweepGenerator(t, outputDir)
	outdir := filepath.Join(outputDir, "default")
	require.NoError(t, os.MkdirAll(outdir, xfs.DefaultDirPerm))

	handwritten := filepath.Join(outdir, "handwritten_base.proto")
	require.NoError(t, os.WriteFile(handwritten, []byte(`syntax = "proto3";
package protoconf;
message HandBase { uint32 id = 1; }
`), xfs.DefaultFilePerm))

	// nothing is generated in this run, aka all workbooks were deleted
	require.NoError(t, gen.output.createStagingDir())
	defer gen.output.removeStagingDir()
	require.NoError(t, gen.output.publishAll())

	assert.FileExists(t, handwritten, "handwritten proto in outdir must never be swept")
}

func TestProtoOutput_publishRemovesStaleProto(t *testing.T) {
	outputDir := t.TempDir()
	gen := newSweepGenerator(t, outputDir)
	outdir := filepath.Join(outputDir, "default")
	require.NoError(t, os.MkdirAll(outdir, xfs.DefaultDirPerm))

	header := generatedFileHeaderLine()
	kept := filepath.Join(outdir, "item_conf.proto")
	stale := filepath.Join(outdir, "stale_conf.proto")
	require.NoError(t, os.WriteFile(kept, []byte(header), xfs.DefaultFilePerm))
	require.NoError(t, os.WriteFile(stale, []byte(header), xfs.DefaultFilePerm))

	require.NoError(t, gen.output.reservePath(kept, "Item.xlsx"))
	require.NoError(t, gen.output.createStagingDir())
	defer gen.output.removeStagingDir()
	require.NoError(t, gen.output.publishAll())

	assert.FileExists(t, kept, "proto generated in this run must be kept")
	assert.NoFileExists(t, stale, "proto not generated in this run must be swept")
}

func TestProtoOutput_reservePathConflict(t *testing.T) {
	gen := newSweepGenerator(t, t.TempDir())
	path := filepath.Join(gen.OutputDir, "default", "shop_conf.proto")

	require.NoError(t, gen.output.reservePath(path, "conf/server/Shop.xlsx"))

	err := gen.output.reservePath(path, "conf/server/Activity/Shop.xlsx")
	require.Error(t, err)
	assert.ErrorIs(t, err, xerrors.ErrE1000)
	// NOTE: the two conflicting workbooks share the same basename, so the error
	// must name their paths to tell them apart.
	assert.ErrorContains(t, err, "conf/server/Shop.xlsx")
	assert.ErrorContains(t, err, "conf/server/Activity/Shop.xlsx")
}

func TestProtoOutput_reservePathDistinctPaths(t *testing.T) {
	gen := newSweepGenerator(t, t.TempDir())
	outdir := filepath.Join(gen.OutputDir, "default")

	require.NoError(t, gen.output.reservePath(filepath.Join(outdir, "item_conf.proto"), "Item.xlsx"))
	require.NoError(t, gen.output.reservePath(filepath.Join(outdir, "skill_conf.proto"), "Skill.xlsx"))
}
