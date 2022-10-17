package protogen

import (
	"bytes"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/protogen/parseroptions"
	"github.com/tableauio/tableau/internal/xproto"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"golang.org/x/sync/errgroup"
)

const (
	TableauProtoPackage = "tableau"
	defaultTopN         = 10 // default top N rows for importer's TopN option
)

type Generator struct {
	ProtoPackage string // protobuf package name.
	InputDir     string // input dir of workbooks.
	OutputDir    string // output dir of generated protoconf files.

	LocationName string // TZ location name.
	InputOpt     *options.InputProtoOption
	OutputOpt    *options.OutputProtoOption

	// internal
	fileDescs []*desc.FileDescriptor      // all parsed imported proto file descriptors.
	typeInfos map[string]*xproto.TypeInfo // proto full type name -> type info
}

func NewGenerator(protoPackage, indir, outdir string, setters ...options.Option) *Generator {
	opts := options.ParseOptions(setters...)
	return NewGeneratorWithOptions(protoPackage, indir, outdir, opts)
}

func NewGeneratorWithOptions(protoPackage, indir, outdir string, opts *options.Options) *Generator {
	g := &Generator{
		ProtoPackage: protoPackage,
		InputDir:     indir,
		OutputDir:    outdir,
		LocationName: opts.LocationName,
		InputOpt:     opts.Input.Proto,
		OutputOpt:    opts.Output.Proto,
	}

	// parse custom imported proto files
	fileDescs, err := xproto.ParseProtos(
		g.InputOpt.ProtoPaths,
		g.InputOpt.ImportedProtoFiles...)
	if err != nil {
		log.Panic(err)
	}
	g.fileDescs = fileDescs
	g.typeInfos = xproto.GetAllTypeInfo(fileDescs)

	return g
}

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

func (gen *Generator) Generate(relWorkbookPaths ...string) error {
	if len(relWorkbookPaths) == 0 {
		return gen.GenAll()
	}
	return gen.GenWorkbook(relWorkbookPaths...)
}

func (gen *Generator) GenAll() error {
	outputProtoDir := filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir)
	if err := prepareOutdir(outputProtoDir, gen.InputOpt.ImportedProtoFiles, true); err != nil {
		return err
	}
	if len(gen.InputOpt.Subdirs) != 0 {
		for _, subdir := range gen.InputOpt.Subdirs {
			dir := filepath.Join(gen.InputDir, subdir)
			if err := gen.generate(dir); err != nil {
				return err
			}
		}
		return nil
	}
	return gen.generate(gen.InputDir)
}

func (gen *Generator) GenWorkbook(relWorkbookPaths ...string) error {
	outputProtoDir := filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir)
	if err := prepareOutdir(outputProtoDir, gen.InputOpt.ImportedProtoFiles, false); err != nil {
		return err
	}
	var eg errgroup.Group
	for _, relWorkbookPath := range relWorkbookPaths {
		absPath := filepath.Join(gen.InputDir, relWorkbookPath)
		eg.Go(func() error {
			return gen.convertWithErrorModule(filepath.Dir(absPath), filepath.Base(absPath), false)
		})
	}
	return eg.Wait()
}

func (gen *Generator) generate(dir string) (err error) {
	var eg errgroup.Group
	defer func() {
		if err == nil {
			err = eg.Wait()
		}
	}()

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return xerrors.WrapKV(err, xerrors.KeyIndir, gen.InputDir)
	}

	// book name -> existence(bool)
	csvBooks := map[string]bool{}
	for _, entry := range dirEntries {
		if entry.IsDir() {
			// scan and generate subdir recursively
			subdir := filepath.Join(dir, entry.Name())
			err = gen.generate(subdir)
			if err != nil {
				return xerrors.WithMessageKV(err, xerrors.KeySubdir, subdir)
			}
			continue
		} else if gen.InputOpt.FollowSymlink && entry.Type() == iofs.ModeSymlink {
			dstPath, err := os.Readlink(filepath.Join(dir, entry.Name()))
			if err != nil {
				return xerrors.WrapKV(err)
			}
			fileInfo, err := os.Stat(dstPath)
			if err != nil {
				return xerrors.WrapKV(err)
			}

			if !fileInfo.IsDir() {
				// is not a directory
				log.Warnf("symlink: %s is not a directory, currently not processed", dstPath)
			}
			err = gen.generate(dstPath)
			if err != nil {
				return xerrors.WithMessageKV(err, xerrors.KeySubdir, dstPath)
			}
			continue
		}

		if strings.HasPrefix(entry.Name(), "~$") {
			// ignore temp file named with prefix "~$"
			continue
		}
		// log.Debugf("generating %s, %s", entry.Name(), filepath.Ext(entry.Name()))
		fmt := format.Ext2Format(filepath.Ext(entry.Name()))
		// check if this workbook format need to be converted
		if !format.FilterInput(fmt, gen.InputOpt.Formats) {
			continue
		}

		if fmt == format.CSV {
			bookName, _, err := importer.ParseCSVFilenamePattern(entry.Name())
			if err != nil {
				return err
			}
			if _, ok := csvBooks[bookName]; ok {
				// NOTE: multiple CSV files construct the same book.
				continue
			}
			csvBooks[bookName] = true
		}

		filename := entry.Name()
		eg.Go(func() error {
			return gen.convertWithErrorModule(dir, filename, true)
		})
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

func (gen *Generator) convertWithErrorModule(dir, filename string, checkProtoFileConflicts bool) error {
	if err := gen.convert(dir, filename, checkProtoFileConflicts); err != nil {
		return xerrors.WithMessageKV(err, xerrors.KeyModule, xerrors.ModuleProto)
	}
	return nil
}

func (gen *Generator) convert(dir, filename string, checkProtoFileConflicts bool) (err error) {
	absPath := filepath.Join(dir, filename)
	parser := confgen.NewSheetParser(TableauProtoPackage, gen.LocationName, book.MetasheetOptions())
	imp, err := importer.New(absPath, importer.Parser(parser), importer.TopN(defaultTopN))
	if err != nil {
		return xerrors.WrapKV(err, xerrors.KeyBookName, absPath)
	}

	sheets := imp.GetSheets()
	if len(sheets) == 0 {
		return nil
	}
	basename := filepath.Base(imp.Filename())
	relativePath, err := getRelCleanSlashPath(gen.InputDir, dir, basename)
	if err != nil {
		return err
	}
	debugWorkbookName := relativePath
	// rewrite subdir
	rewrittenWorkbookName := fs.RewriteSubdir(relativePath, gen.InputOpt.SubdirRewrites)
	if rewrittenWorkbookName != relativePath {
		debugWorkbookName += " (rewrite: " + rewrittenWorkbookName + ")"
	}
	log.Infof("%18s: %s, %d worksheet(s) will be parsed", "analyzing workbook", debugWorkbookName, len(sheets))
	// creat a book parser
	bp := newBookParser(imp.BookName(), rewrittenWorkbookName, gen)
	for _, sheet := range sheets {
		// parse sheet header
		debugSheetName := sheet.Name
		sheetMsgName := sheet.Name
		if sheet.Meta.Alias != "" {
			sheetMsgName = sheet.Meta.Alias
			debugSheetName += " (alias: " + sheet.Meta.Alias + ")"
		}
		log.Infof("%18s: %s", "parsing worksheet", debugSheetName)
		mergeHeaderOptions(sheet.Meta, gen.InputOpt.Header)
		ws := &tableaupb.Worksheet{
			Options: &tableaupb.WorksheetOptions{
				Name:          sheet.Name,
				Namerow:       sheet.Meta.Namerow,
				Typerow:       sheet.Meta.Typerow,
				Noterow:       sheet.Meta.Noterow,
				Datarow:       sheet.Meta.Datarow,
				Transpose:     sheet.Meta.Transpose,
				Tags:          "",
				Nameline:      sheet.Meta.Nameline,
				Typeline:      sheet.Meta.Typeline,
				Nested:        sheet.Meta.Nested,
				Sep:           sheet.Meta.Sep,
				Subsep:        sheet.Meta.Subsep,
				Merger:        sheet.Meta.Merger,
				AdjacentKey:   sheet.Meta.AdjacentKey,
				FieldPresence: sheet.Meta.FieldPresence,
				Template:      sheet.Meta.Template,
				// Loader options:
				OrderedMap: sheet.Meta.OrderedMap,
				Index:      sheet.Meta.Index,
			},
			Fields: []*tableaupb.Field{},
			Name:   sheetMsgName,
		}

		shHeader := &sheetHeader{
			meta: sheet.Meta,
		}
		// transpose or not
		if sheet.Meta.Transpose {
			for row := 0; row < sheet.MaxRow; row++ {
				nameCol := int(sheet.Meta.Namerow) - 1
				nameCell, err := sheet.Cell(row, nameCol)
				if err != nil {
					return xerrors.WithMessageKV(err, xerrors.KeyBookName, debugWorkbookName, xerrors.KeySheetName, debugSheetName, xerrors.KeyNameCellPos, excel.Postion(row, nameCol))
				}
				shHeader.namerow = append(shHeader.namerow, nameCell)

				typeCol := int(sheet.Meta.Typerow) - 1
				typeCell, err := sheet.Cell(row, typeCol)
				if err != nil {
					return xerrors.WithMessageKV(err, xerrors.KeyBookName, debugWorkbookName, xerrors.KeySheetName, debugSheetName, xerrors.KeyNameCellPos, excel.Postion(row, typeCol))
				}
				shHeader.typerow = append(shHeader.typerow, typeCell)

				noteCol := int(sheet.Meta.Noterow) - 1
				noteCell, err := sheet.Cell(row, noteCol)
				if err != nil {
					return xerrors.WithMessageKV(err, xerrors.KeyBookName, debugWorkbookName, xerrors.KeySheetName, debugSheetName, xerrors.KeyNameCellPos, excel.Postion(row, noteCol))
				}
				shHeader.noterow = append(shHeader.noterow, noteCell)
			}
		} else {
			shHeader.namerow = sheet.Rows[sheet.Meta.Namerow-1]
			shHeader.typerow = sheet.Rows[sheet.Meta.Typerow-1]
			shHeader.noterow = sheet.Rows[sheet.Meta.Noterow-1]
		}

		var parsed bool
		for cursor := 0; cursor < len(shHeader.namerow); cursor++ {
			field := &tableaupb.Field{}
			cursor, parsed, err = bp.parseField(field, shHeader, cursor, "", parseroptions.Nested(sheet.Meta.Nested))
			if err != nil {
				nameCellPos := excel.Postion(int(sheet.Meta.Namerow-1), cursor)
				typeCellPos := excel.Postion(int(sheet.Meta.Typerow-1), cursor)
				if sheet.Meta.Transpose {
					nameCellPos = excel.Postion(cursor, int(sheet.Meta.Namerow-1))
					typeCellPos = excel.Postion(cursor, int(sheet.Meta.Typerow-1))
				}
				return xerrors.WithMessageKV(err,
					xerrors.KeyBookName, debugWorkbookName,
					xerrors.KeySheetName, debugSheetName,
					xerrors.KeyNameCellPos, nameCellPos,
					xerrors.KeyTypeCellPos, typeCellPos,
					xerrors.KeyNameCell, shHeader.getNameCell(cursor),
					xerrors.KeyTypeCell, shHeader.getTypeCell(cursor))
			}
			if parsed {
				ws.Fields = append(ws.Fields, field)
			}
		}
		// append parsed sheet to workbook
		bp.wb.Worksheets = append(bp.wb.Worksheets, ws)
	}
	// export book
	be := newBookExporter(
		gen.ProtoPackage,
		gen.OutputOpt.FileOptions,
		filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir),
		gen.OutputOpt.FilenameSuffix,
		bp.wb,
		bp.gen,
	)
	if err := be.export(checkProtoFileConflicts); err != nil {
		return xerrors.WithMessageKV(err, xerrors.KeyBookName, debugWorkbookName)
	}

	return nil
}

type sheetHeader struct {
	meta    *tableaupb.Metasheet
	namerow []string
	typerow []string
	noterow []string
}

func getCell(row []string, cursor int, line int32) string {
	// empty cell may be not in list
	if cursor >= len(row) {
		return ""
	}
	return book.ExtractFromCell(row[cursor], line)
}

func (sh *sheetHeader) getNameCell(cursor int) string {
	return getCell(sh.namerow, cursor, sh.meta.Nameline)
}

// getValidNameCell try best to get a none-empty cell, starting from
// the specified cursor. Current and subsequent empty cells are skipped
// to find the first none-empty name cell.
func (sh *sheetHeader) getValidNameCell(cursor *int) string {
	for *cursor < len(sh.namerow) {
		cell := getCell(sh.namerow, *cursor, sh.meta.Nameline)
		if cell == "" {
			*cursor++
			continue
		}
		return cell
	}
	return ""
}

func (sh *sheetHeader) getTypeCell(cursor int) string {
	return getCell(sh.typerow, cursor, sh.meta.Typeline)
}
func (sh *sheetHeader) getNoteCell(cursor int) string {
	return getCell(sh.noterow, cursor, 1) // default note line is 1
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
