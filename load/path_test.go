package load

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestExpandPatchPaths_literalAndGlob(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a_1.json"), []byte("{}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a_2.json"), []byte("{}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.json"), []byte("{}"), 0o644))

	got, err := expandPatchPaths(hostFS{}, dir, []string{"b.json", "a_*.json", ""})
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(dir, "b.json"),
		filepath.Join(dir, "a_1.json"),
		filepath.Join(dir, "a_2.json"),
	}, got)
}

func TestExpandPatchPaths_noMatch(t *testing.T) {
	dir := t.TempDir()

	got, err := expandPatchPaths(hostFS{}, dir, []string{"none_*.json"})
	require.Error(t, err)
	require.Nil(t, got)
	require.Contains(t, err.Error(), "matched no files")

	got, err = expandPatchPaths(hostFS{}, dir, []string{"missing.json"})
	require.Error(t, err)
	require.Nil(t, got)
	require.Contains(t, err.Error(), "matched no files")
}

func TestExpandPatchPaths_reuseAcrossDirs(t *testing.T) {
	pattern := "shard_*.binpb"
	oldDir := t.TempDir()
	newDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(oldDir, "shard_1.binpb"), []byte{1}, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(newDir, "shard_1.binpb"), []byte{1}, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(newDir, "shard_2.binpb"), []byte{1}, 0o644))

	oldMatches, err := expandPatchPaths(hostFS{}, oldDir, []string{pattern})
	require.NoError(t, err)
	require.Equal(t, []string{filepath.Join(oldDir, "shard_1.binpb")}, oldMatches)

	newMatches, err := expandPatchPaths(hostFS{}, newDir, []string{pattern})
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(newDir, "shard_1.binpb"),
		filepath.Join(newDir, "shard_2.binpb"),
	}, newMatches)
}

func TestExpandPatchPaths_withFileSystem(t *testing.T) {
	fsys := fstest.MapFS{
		"b.json":   {Data: []byte("{}")},
		"a_1.json": {Data: []byte("{}")},
		"a_2.json": {Data: []byte("{}")},
	}
	got, err := expandPatchPaths(fsys, ".", []string{"b.json", "a_*.json"})
	require.NoError(t, err)
	require.Equal(t, []string{"b.json", "a_1.json", "a_2.json"}, got)
}
