package protogen

import (
	"os"
	"path/filepath"
	"strings"

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

func wrapDebugErr(err error, bookName, sheetName string, header *tableHeader, cursor int) error {
	getCellPos := func(row int) string {
		return header.Position(row-1, cursor)
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
