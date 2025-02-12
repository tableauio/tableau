package protogen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/xfs"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"github.com/tableauio/tableau/xerrors"
)

type parsePass int

const (
	firstPass  parsePass = iota // only generate type definitions from sheets
	secondPass                  // generate config messagers from sheets
)

const (
	colNumber = "Number" // name of column "Number"
	colName   = "Name"   // name of column "Name"
	colAlias  = "Alias"  // name of column "Alias"
)

func isEnumTypeDefinitionBlockHeader(cols []string) bool {
	if len(cols) < 3 {
		return false
	}
	return cols[0] == colNumber && cols[1] == colName && cols[2] == colAlias
}

// extractEnumTypeRow find the first none-empty colunm as "name", and then
// the subsequent column as "alias".
func extractEnumTypeRow(cols []string) (name, alias string, err error) {
	for i, cell := range cols {
		if cell != "" {
			name = cell
			if i+1 < len(cols) {
				alias = cols[i+1]
			}
			break
		}
	}
	if name == "" {
		return name, alias, fmt.Errorf("name cell not found in enum type row")
	}
	return
}

func parseEnumTypeValues(ws *internalpb.Worksheet, sheet *book.Sheet, parser book.SheetParser) error {
	desc := &internalpb.EnumDescriptor{}
	if err := parser.Parse(desc, sheet); err != nil {
		return xerrors.Wrapf(err, "failed to parse enum type sheet: %s", sheet.Name)
	}
	for i, value := range desc.Values {
		number := int32(i + 1)
		if value.Number != nil {
			number = *value.Number
		}
		field := &internalpb.Field{
			Number: number,
			Name:   value.Name,
			Alias:  value.Alias,
		}
		ws.Fields = append(ws.Fields, field)
	}
	return nil
}

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

// mergeHeaderOptions merge from options.HeaderOption to internalpb.Metasheet.
func mergeHeaderOptions(sheetOpts *tableaupb.WorksheetOptions, headerOpt *options.HeaderOption) {
	if headerOpt == nil {
		return
	}

	if sheetOpts.Namerow == 0 {
		sheetOpts.Namerow = headerOpt.Namerow
	}
	if sheetOpts.Typerow == 0 {
		sheetOpts.Typerow = headerOpt.Typerow
	}
	if sheetOpts.Noterow == 0 {
		sheetOpts.Noterow = headerOpt.Noterow
	}
	if sheetOpts.Datarow == 0 {
		sheetOpts.Datarow = headerOpt.Datarow
	}
	if sheetOpts.Nameline == 0 {
		sheetOpts.Nameline = headerOpt.Nameline
	}
	if sheetOpts.Typeline == 0 {
		sheetOpts.Typeline = headerOpt.Typeline
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

func wrapDebugErr(err error, bookName, sheetName string, sh *tableHeader, cursor int) error {
	nameCellPos := excel.Postion(int(sh.meta.Namerow-1), cursor)
	typeCellPos := excel.Postion(int(sh.meta.Typerow-1), cursor)
	if sh.meta.Transpose {
		nameCellPos = excel.Postion(cursor, int(sh.meta.Namerow-1))
		typeCellPos = excel.Postion(cursor, int(sh.meta.Typerow-1))
	}
	return xerrors.WrapKV(err,
		xerrors.KeyBookName, bookName,
		xerrors.KeySheetName, sheetName,
		xerrors.KeyNameCellPos, nameCellPos,
		xerrors.KeyTypeCellPos, typeCellPos,
		xerrors.KeyNameCell, sh.getNameCell(cursor),
		xerrors.KeyTypeCell, sh.getTypeCell(cursor))
}
