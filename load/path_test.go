package load

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveConfigPath(t *testing.T) {
	dir := filepath.FromSlash("/cfg/v1")

	require.Equal(t, "", resolveConfigPath(dir, ""))
	require.Equal(t, filepath.Join(dir, "ItemConf.json"), resolveConfigPath(dir, "ItemConf.json"))
	require.Equal(t, filepath.Join(dir, "patch", "ItemConf.json"), resolveConfigPath(dir, "patch/ItemConf.json"))
	require.Equal(t, filepath.Join(dir, "..", "patch", "X.json"), resolveConfigPath(dir, "../patch/X.json"))

	abs := filepath.FromSlash("/abs/ItemConf.json")
	require.Equal(t, abs, resolveConfigPath(dir, abs))
}

func TestResolveConfigPath_ReuseAcrossDirs(t *testing.T) {
	// Same relative Path resolves under different load dirs — the
	// CheckCompatibility pattern (one MessagerOptions, two trees).
	rel := "scatter/ItemConf.binpb"
	require.Equal(t,
		filepath.Join("/old", rel),
		resolveConfigPath("/old", rel),
	)
	require.Equal(t,
		filepath.Join("/new", rel),
		resolveConfigPath("/new", rel),
	)
}
