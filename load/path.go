package load

import (
	"io/fs"
	"path/filepath"

	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/internal/x/xfs"
)

// expandPatchPaths resolves each pattern against dir, expands with fs.Glob on
// fsys (literals are valid patterns), and errors if any pattern matches nothing.
func expandPatchPaths(fsys fs.FS, dir string, patterns []string) ([]string, error) {
	out := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		resolved := xfs.ResolveAgainstDir(dir, pattern)
		matches, err := fs.Glob(fsys, filepath.ToSlash(resolved))
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
