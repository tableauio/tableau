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

// readFuncFS adapts a [ReadFunc] to [fs.FS].
// ReadFile delegates to the ReadFunc (with OS-native path for os.ReadFile
// compatibility). Open/Glob use hostFS so PatchPaths globs still see the host.
type readFuncFS struct {
	read ReadFunc
}

var (
	_ fs.FS         = readFuncFS{}
	_ fs.ReadFileFS = readFuncFS{}
	_ fs.GlobFS     = readFuncFS{}
)

func (f readFuncFS) Open(name string) (fs.File, error) {
	return hostFS{}.Open(name)
}

func (f readFuncFS) ReadFile(name string) ([]byte, error) {
	return f.read(filepath.FromSlash(name))
}

func (f readFuncFS) Glob(pattern string) ([]string, error) {
	return hostFS{}.Glob(pattern)
}
