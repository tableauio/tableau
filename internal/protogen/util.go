package protogen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/xerrors"
)

type parsePass int

const (
	firstPass  parsePass = iota // only generate type definitions from sheets
	secondPass                  // generate config messagers from sheets
)

func prepareOutdir(outdir string, importFiles []string, delExisted bool) error {
	existed, err := xfs.Exists(outdir)
	if err != nil {
		return xerrors.WrapKV(err, xerrors.KeyOutdir, outdir)
	}
	if existed && delExisted {
		// remove all *.proto file but not Imports
		imports := make(map[string]int)
		for _, path := range importFiles {
			imports[path] = 1
		}
		files, err := os.ReadDir(outdir)
		if err != nil {
			return xerrors.WrapKV(err, xerrors.KeyOutdir, outdir)
		}
		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".proto") {
				continue
			}
			if _, ok := imports[file.Name()]; ok {
				continue
			}
			fpath := filepath.Join(outdir, file.Name())
			err := os.Remove(fpath)
			if err != nil {
				return xerrors.WrapKV(err)
			}
		}
	} else {
		// create output dir
		err = os.MkdirAll(outdir, xfs.DefaultDirPerm)
		if err != nil {
			return xerrors.WrapKV(err, xerrors.KeyOutdir, outdir)
		}
	}

	return nil
}

func getRelCleanSlashPath(rootdir, dir, filename string) (string, error) {
	relativeDir, err := filepath.Rel(rootdir, dir)
	if err != nil {
		return "", xerrors.Errorf("failed to get relative path from %s to %s: %s", rootdir, dir, err)
	}
	// relative slash separated path
	relativePath := filepath.Join(relativeDir, filename)
	relSlashPath := filepath.ToSlash(filepath.Clean(relativePath))
	return relSlashPath, nil
}

func genProtoFilePath(bookName, suffix string) string {
	return bookName + suffix + ".proto"
}

type GeneratedBuf struct {
	buf bytes.Buffer
}

// NewGeneratedFile creates a new generated file with the given filename.
func NewGeneratedBuf() *GeneratedBuf {
	return &GeneratedBuf{}
}

// P prints a line to the generated output. It converts each parameter to a
// string following the same rules as fmt.Print. It never inserts spaces
// between parameters.
func (g *GeneratedBuf) P(v ...any) {
	for _, x := range v {
		fmt.Fprint(&g.buf, x)
	}
	fmt.Fprintln(&g.buf)
}

// Content returns the contents of the generated file.
func (g *GeneratedBuf) Content() []byte {
	return g.buf.Bytes()
}

// String returns the string content of the generated file.
func (g *GeneratedBuf) String() string {
	return g.buf.String()
}

func wrapDebugErr(err error, bookName, sheetName string, header *tableHeader, cursor int) error {
	nameCellPos := excel.Postion(header.NameRow-1, cursor)
	typeCellPos := excel.Postion(header.TypeRow-1, cursor)
	if header.transpose {
		nameCellPos = excel.Postion(cursor, header.NameRow-1)
		typeCellPos = excel.Postion(cursor, header.TypeRow-1)
	}
	return xerrors.WrapKV(err,
		xerrors.KeyBookName, bookName,
		xerrors.KeySheetName, sheetName,
		xerrors.KeyNameCellPos, nameCellPos,
		xerrors.KeyTypeCellPos, typeCellPos,
		xerrors.KeyNameCell, header.getNameCell(cursor),
		xerrors.KeyTypeCell, header.getTypeCell(cursor))
}
