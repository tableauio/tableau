package protogen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/log"
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
			log.Debugf("remove existed proto file: %s", fpath)
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
	getCellPos := func(row int) string {
		if header.transpose {
			return excel.Postion(cursor, row-1)
		}
		return excel.Postion(row-1, cursor)
	}
	return xerrors.WrapKV(err,
		xerrors.KeyBookName, bookName,
		xerrors.KeySheetName, sheetName,
		xerrors.KeyNameCellPos, getCellPos(header.NameRow),
		xerrors.KeyTypeCellPos, getCellPos(header.TypeRow),
		xerrors.KeyNoteCellPos, getCellPos(header.NoteRow),
		xerrors.KeyNameCell, header.getNameCell(cursor),
		xerrors.KeyTypeCell, header.getTypeCell(cursor),
		xerrors.KeyNoteCell, header.getNoteCell(cursor),
	)
}

var (
	safeFieldNameReg         = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	safeEnumNameReg          = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	safeMessageNameReg       = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
	consecutiveUnderscoreReg = regexp.MustCompile(`__+`)
)

func safeFieldName(fieldName string, existingFieldNames map[string]bool) (string, error) {
	// Trim spaces
	fieldName = strings.TrimSpace(fieldName)

	// Validate field name
	ok := safeFieldNameReg.MatchString(fieldName)
	if !ok {
		return "", xerrors.Errorf("invalid field name: %s", fieldName)
	}

	// Remove consecutive underscores
	fieldName = consecutiveUnderscoreReg.ReplaceAllString(fieldName, "_")

	// Trim underscores from end
	fieldName = strings.TrimSuffix(fieldName, "_")

	// Check for duplicates
	if existingFieldNames[fieldName] {
		i := 1
		for ; existingFieldNames[fieldName+strconv.Itoa(i)]; i++ {
		}
		fieldName += strconv.Itoa(i)
	}

	existingFieldNames[fieldName] = true

	return fieldName, nil
}

func safeEnumName(enumName string, existingEnumNames map[string]bool) (string, error) {
	// Trim spaces
	enumName = strings.TrimSpace(enumName)

	// Validate enum name
	ok := safeEnumNameReg.MatchString(enumName)
	if !ok {
		return "", xerrors.Errorf("invalid enum name: %s", enumName)
	}

	// Remove consecutive underscores
	enumName = consecutiveUnderscoreReg.ReplaceAllString(enumName, "_")

	// Trim underscores from end
	enumName = strings.TrimSuffix(enumName, "_")

	// Check for duplicates
	if existingEnumNames[enumName] {
		i := 1
		for ; existingEnumNames[enumName+strconv.Itoa(i)]; i++ {
		}
		enumName += strconv.Itoa(i)
	}

	existingEnumNames[enumName] = true

	return enumName, nil
}

func safeMessageName(messageName string, existingMessageNames map[string]bool) (string, error) {
	// Trim spaces
	messageName = strings.TrimSpace(messageName)

	// Validate enum name
	ok := safeMessageNameReg.MatchString(messageName)
	if !ok {
		return "", xerrors.Errorf("invalid enum name: %s", messageName)
	}

	// Remove consecutive underscores
	messageName = consecutiveUnderscoreReg.ReplaceAllString(messageName, "_")

	// Trim underscores from end
	messageName = strings.TrimSuffix(messageName, "_")

	// Check for duplicates
	if existingMessageNames[messageName] {
		i := 1
		for ; existingMessageNames[messageName+strconv.Itoa(i)]; i++ {
		}
		messageName += strconv.Itoa(i)
	}

	existingMessageNames[messageName] = true

	return messageName, nil
}
