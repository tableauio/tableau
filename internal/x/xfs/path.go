package xfs

import (
	"path/filepath"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

// Dir returns all but the last element of path, typically the path's directory.
// The result is a clean and slash path.
func Dir(path string) string {
	dir := filepath.Dir(path)
	return CleanSlashPath(dir)
}

// Join joins any number of path elements into a clean and slash path.
func Join(elem ...string) string {
	path := filepath.Join(elem...)
	return CleanSlashPath(path)
}

// CleanSlashPath returns clean and slash path.
func CleanSlashPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

// abs is the function used to resolve absolute paths. It is a package-level
// variable so that tests can replace it with a mock without touching the
// filesystem.
var abs = filepath.Abs

// IsSamePath reports whether leftPath and rightPath refer to the same file-system
// location. Both paths are first resolved to absolute paths (relative paths are
// resolved against the current working directory) and then cleaned and
// normalized to forward-slash form before comparison, so mixed relative/absolute
// or dot-segment paths that point to the same location are considered equal.
func IsSamePath(leftPath, rightPath string) bool {
	leftAbs, err := abs(leftPath)
	if err != nil {
		return CleanSlashPath(leftPath) == CleanSlashPath(rightPath)
	}
	rightAbs, err := abs(rightPath)
	if err != nil {
		return CleanSlashPath(leftPath) == CleanSlashPath(rightPath)
	}
	return CleanSlashPath(leftAbs) == CleanSlashPath(rightAbs)
}

// Rel returns relative clean slash path.
func Rel(basepath string, targetpath string) (string, error) {
	relPath, err := filepath.Rel(basepath, targetpath)
	if err != nil {
		return "", xerrors.Wrapf(err, "failed to get relative path from %s to %s", basepath, targetpath)
	}
	return CleanSlashPath(relPath), nil
}
