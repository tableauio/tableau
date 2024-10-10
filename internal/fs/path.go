package fs

import (
	"path/filepath"

	"github.com/tableauio/tableau/xerrors"
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

// IsSamePath checks if two paths are same based on clean slash path.
func IsSamePath(leftPath, rightPath string) bool {
	return CleanSlashPath(leftPath) == CleanSlashPath(rightPath)
}

// Rel returns relative clean slash path.
func Rel(basepath string, targetpath string) (string, error) {
	relPath, err := filepath.Rel(basepath, targetpath)
	if err != nil {
		return "", xerrors.Wrapf(err, "failed to get relative path from %s to %s", basepath, targetpath)
	}
	return CleanSlashPath(relPath), nil
}
