package load

import (
	"io/fs"
	"path/filepath"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

// resolveConfigPath resolves a config file path against the load directory.
//
// Semantics:
//   - empty: caller should fall back to dir/Name.ext (not handled here)
//   - absolute: used as-is
//   - relative: joined with dir (filepath.Join(dir, path)) — ".." may escape dir
//
// This lets one MessagerOptions be reused across different load dirs
// (e.g. CheckCompatibility old vs new), while still allowing absolute overrides.
func resolveConfigPath(dir, path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dir, path)
}

// expandPatchPaths resolves each PatchPaths entry against dir, then expands it
// with fs.Glob on opts.GetFS() (default hostFS). Literal paths are valid Glob
// patterns. Zero matches is an error. Matches stay in pattern order; each
// Glob result is already sorted by the standard library.
func expandPatchPaths(opts *MessagerOptions, dir string, patterns []string) ([]string, error) {
	out := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		resolved := resolveConfigPath(dir, pattern)
		matches, err := fs.Glob(opts.GetFS(), filepath.ToSlash(resolved))
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			return nil, xerrors.Newf("PatchPaths glob %q matched no files", resolved)
		}
		out = append(out, matches...)
	}
	return out, nil
}

// readConfigFile reads via opts.GetFS() (default hostFS).
func readConfigFile(opts *MessagerOptions, pathName string) ([]byte, error) {
	return fs.ReadFile(opts.GetFS(), filepath.ToSlash(pathName))
}
