package load

import "path/filepath"

// resolveConfigPath resolves a config file path against the load directory.
//
// Semantics:
//   - empty: caller should fall back to dir/Name.ext (not handled here)
//   - absolute: used as-is
//   - relative: joined with dir (filepath.Join(dir, path))
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
