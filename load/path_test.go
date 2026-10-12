package load

import (
	"path/filepath"
	"testing"

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
