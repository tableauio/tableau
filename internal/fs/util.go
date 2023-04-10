package fs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
)

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherise, attempt to create a hard link
// between the two files. If that fail, copy the file contents from src to dst.
func CopyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	if err = os.Link(src, dst); err == nil {
		return
	}
	err = copyFileContents(src, dst)
	return
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
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

func GetCleanSlashPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func IsSamePath(leftPath, rightPath string) bool {
	return GetCleanSlashPath(leftPath) == GetCleanSlashPath(rightPath)
}

func GetRelCleanSlashPath(basepath string, targetpath string) (string, error) {
	relPath, err := filepath.Rel(basepath, targetpath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get relative path from %s to %s", basepath, targetpath)
	}
	return GetCleanSlashPath(relPath), nil
}

func FilterSubdir(filename string, subdirs []string) bool {
	if len(subdirs) != 0 {
		for _, subdir := range subdirs {
			subdir = GetCleanSlashPath(subdir)
			if strings.HasPrefix(filename, subdir) {
				return true
			}
		}
		return false
	}
	return true
}

func RewriteSubdir(filename string, subdirRewrites map[string]string) string {
	if len(subdirRewrites) != 0 {
		for old, new := range subdirRewrites {
			oldSubdir := GetCleanSlashPath(old)
			newSubdir := GetCleanSlashPath(new)
			if strings.HasPrefix(filename, oldSubdir) {
				newfilename := strings.Replace(filename, oldSubdir, newSubdir, 1)
				return GetCleanSlashPath(newfilename)
			}
		}
	}
	return filename
}

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
		fileFmt := format.Ext2Format(filepath.Ext(entry.Name()))
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
			return errors.New("unknown fommat: " + string(fmt))
		}
	}
	return nil
}
