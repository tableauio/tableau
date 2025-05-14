package xfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/xerrors"
)

const (
	DefaultDirPerm  fs.FileMode = 0755 // drwxr-xr-x
	DefaultFilePerm fs.FileMode = 0644 // -rw-r--r--
)

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherise, attempt to create a hard link
// between the two files. If that fails, copy the file contents from src to dst.
func CopyFile(src, dst string) error {
	srcFile, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcFile.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", srcFile.Name(), srcFile.Mode().String())
	}
	dstFile, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		if !(dstFile.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dstFile.Name(), dstFile.Mode().String())
		}
		if os.SameFile(srcFile, dstFile) {
			return nil
		}
	}
	if err = os.Link(src, dst); err == nil {
		return nil
	}
	return copyFileContents(src, dst)
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		cerr := in.Close()
		err = errors.Join(err, cerr)
	}()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		cerr := out.Close()
		err = errors.Join(err, cerr)
	}()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// Exists returns whether the given file or directory exists
func Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// HasSubdirPrefix returns whether the given path has the given prefix.
func HasSubdirPrefix(path string, subdirs []string) bool {
	if len(subdirs) != 0 {
		path = CleanSlashPath(path)
		for _, subdir := range subdirs {
			subdir = CleanSlashPath(subdir)
			if strings.HasPrefix(path, subdir) {
				return true
			}
		}
		return false
	}
	return true
}

// RewriteSubdir replaces path's subdir part with the given subdirs.
func RewriteSubdir(path string, subdirRewrites map[string]string) string {
	if len(subdirRewrites) != 0 {
		path = CleanSlashPath(path)
		for old, new := range subdirRewrites {
			oldSubdir := CleanSlashPath(old)
			newSubdir := CleanSlashPath(new)
			if strings.HasPrefix(path, oldSubdir) {
				newpath := strings.Replace(path, oldSubdir, newSubdir, 1)
				return CleanSlashPath(newpath)
			}
		}
	}
	return path
}

// RangeFilesByFormat traveses the given directory with the given format, and
// invoke the given callback for each file.
func RangeFilesByFormat(dir string, fmt format.Format, callback func(bookPath string) error) error {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// book name -> existence(bool)
	csvBooks := map[string]bool{}
	for _, entry := range dirEntries {
		if entry.IsDir() {
			// scan and generate subdir recursively
			subdir := filepath.Join(dir, entry.Name())
			err = RangeFilesByFormat(subdir, fmt, callback)
			if err != nil {
				return err
			}
			continue
		}
		fileFmt := format.GetFormat(entry.Name())
		if fileFmt != fmt {
			continue
		}
		switch fmt {
		case format.Excel:
			bookPath := filepath.Join(dir, entry.Name())
			if err := callback(bookPath); err != nil {
				return err
			}
		case format.CSV:
			bookName, _, err := ParseCSVFilenamePattern(entry.Name())
			if err != nil {
				return err
			}
			if _, ok := csvBooks[bookName]; ok {
				// NOTE: multiple CSV files construct the same book.
				continue
			}
			csvBooks[bookName] = true
			if err := callback(GenCSVBooknamePattern(dir, bookName)); err != nil {
				return err
			}
		default:
			return xerrors.Errorf("unknown fommat: %s", fmt)
		}
	}
	return nil
}

// GetDirectParentDirName returns the name of the direct parent directory
// of the given path. E.g.: "/parent/to/file.txt" -> "to".
func GetDirectParentDirName(path string) string {
	pathDir := filepath.Dir(path)
	// Split the parent directory into components
	components := strings.Split(pathDir, string(filepath.Separator))
	if len(components) != 0 {
		// The last componentis the direct parent directory name
		parentDir := components[len(components)-1]
		if parentDir == "." {
			// Dir returns ".". If the path consists entirely of separators
			return ""
		}
		return parentDir
	}
	return ""
}
