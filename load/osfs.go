package load

import (
	"io/fs"
	"os"
	"path/filepath"
)

// hostFS is the default [fs.FS] used when [MessagerOptions.FS] is nil.
//
// It delegates to the host OS (os.Open / os.ReadFile / filepath.Glob):
// relative names are interpreted from the process working directory, and
// absolute paths and ".." elements are allowed — unlike [os.DirFS], which
// roots a tree and rejects those names via [fs.ValidPath].
type hostFS struct{}

var (
	_ fs.FS         = hostFS{}
	_ fs.ReadFileFS = hostFS{}
	_ fs.GlobFS     = hostFS{}
)

func (hostFS) Open(name string) (fs.File, error) {
	return os.Open(filepath.FromSlash(name))
}

func (hostFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(filepath.FromSlash(name))
}

func (hostFS) Glob(pattern string) ([]string, error) {
	return filepath.Glob(filepath.FromSlash(pattern))
}
