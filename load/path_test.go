package load

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

func TestResolveConfigPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cfg", "v1")

	require.Equal(t, "", resolveConfigPath(dir, ""))
	require.Equal(t, filepath.Join(dir, "ItemConf.json"), resolveConfigPath(dir, "ItemConf.json"))
	require.Equal(t, filepath.Join(dir, "patch", "ItemConf.json"), resolveConfigPath(dir, "patch/ItemConf.json"))
	require.Equal(t, filepath.Join(dir, "..", "patch", "X.json"), resolveConfigPath(dir, "../patch/X.json"))

	// Must be a real absolute path: on Windows, paths like "/abs/..." are not IsAbs.
	abs := filepath.Join(t.TempDir(), "ItemConf.json")
	require.True(t, filepath.IsAbs(abs))
	require.Equal(t, abs, resolveConfigPath(dir, abs))
}

func TestResolveConfigPath_ReuseAcrossDirs(t *testing.T) {
	// Same relative Path resolves under different load dirs — the
	// CheckCompatibility pattern (one MessagerOptions, two trees).
	rel := "scatter/ItemConf.binpb"
	oldDir := filepath.Join(t.TempDir(), "old")
	newDir := filepath.Join(t.TempDir(), "new")
	require.Equal(t, filepath.Join(oldDir, filepath.FromSlash(rel)), resolveConfigPath(oldDir, rel))
	require.Equal(t, filepath.Join(newDir, filepath.FromSlash(rel)), resolveConfigPath(newDir, rel))
}

func TestExpandPatchPaths_literalAndGlob(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a_1.json"), []byte("{}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a_2.json"), []byte("{}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.json"), []byte("{}"), 0o644))

	got, err := expandPatchPaths(nil, dir, []string{"b.json", "a_*.json", ""})
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(dir, "b.json"),
		filepath.Join(dir, "a_1.json"),
		filepath.Join(dir, "a_2.json"),
	}, got)
}

func TestExpandPatchPaths_noMatch(t *testing.T) {
	dir := t.TempDir()

	got, err := expandPatchPaths(nil, dir, []string{"none_*.json"})
	require.Error(t, err)
	require.Nil(t, got)
	require.Contains(t, err.Error(), "matched no files")

	got, err = expandPatchPaths(nil, dir, []string{"missing.json"})
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

	oldMatches, err := expandPatchPaths(nil, oldDir, []string{pattern})
	require.NoError(t, err)
	require.Equal(t, []string{filepath.Join(oldDir, "shard_1.binpb")}, oldMatches)

	newMatches, err := expandPatchPaths(nil, newDir, []string{pattern})
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
	opts := &MessagerOptions{BaseOptions: BaseOptions{FS: fsys}}
	got, err := expandPatchPaths(opts, ".", []string{"b.json", "a_*.json"})
	require.NoError(t, err)
	require.Equal(t, []string{"b.json", "a_1.json", "a_2.json"}, got)
}

func TestReadConfigFile_relativeOnFS(t *testing.T) {
	opts := &MessagerOptions{BaseOptions: BaseOptions{FS: fstest.MapFS{
		"ItemConf.json": {Data: []byte(`{}`)},
	}}}
	got, err := readConfigFile(opts, "ItemConf.json")
	require.NoError(t, err)
	require.Equal(t, []byte(`{}`), got)
}
