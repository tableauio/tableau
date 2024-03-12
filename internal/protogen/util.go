package protogen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
)

type parsePass int

const (
	firstPass  parsePass = iota // only generate type definitions from sheets
	secondPass                  // generate config messagers from sheets
)

func prepareOutdir(outdir string, importFiles []string, delExisted bool) error {
	existed, err := fs.Exists(outdir)
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
		err = os.MkdirAll(outdir, 0700)
		if err != nil {
			return xerrors.WrapKV(err, xerrors.KeyOutdir, outdir)
		}
	}

	return nil
}

// getWorkbookAlias gets the workbook alias from importer.
func getWorkbookAlias(imp importer.Importer) string {
	sheetMap := imp.Metabook().GetMetasheetMap()
	if sheetMap == nil {
		return ""
	}
	meta := sheetMap[book.BookNameInMetasheet]
	return meta.GetAlias()
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

// mergeHeaderOptions merge from options.HeaderOption to tableaupb.Metasheet.
func mergeHeaderOptions(sheetMeta *tableaupb.Metasheet, headerOpt *options.HeaderOption) {
	if sheetMeta.Namerow == 0 {
		sheetMeta.Namerow = headerOpt.Namerow
	}
	if sheetMeta.Typerow == 0 {
		sheetMeta.Typerow = headerOpt.Typerow
	}
	if sheetMeta.Noterow == 0 {
		sheetMeta.Noterow = headerOpt.Noterow
	}
	if sheetMeta.Datarow == 0 {
		sheetMeta.Datarow = headerOpt.Datarow
	}
	if sheetMeta.Nameline == 0 {
		sheetMeta.Nameline = headerOpt.Nameline
	}
	if sheetMeta.Typeline == 0 {
		sheetMeta.Typeline = headerOpt.Typeline
	}
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
func (g *GeneratedBuf) P(v ...interface{}) {
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
	return string(g.buf.Bytes())
}
