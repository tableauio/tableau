package protogen

import (
	"bytes"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/protogen/parseroptions"
	"github.com/tableauio/tableau/internal/xproto"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
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

func prepareOutpuDir(outdir string, importFiles []string, delExsited bool) error {
	existed, err := fs.Exists(outdir)
	if err != nil {
		return errors.Wrapf(err, "failed to check existence of output dir: %s", outdir)
	}
	if existed && delExsited {
		// remove all *.proto file but not Imports
		imports := make(map[string]int)
		for _, path := range importFiles {
			imports[path] = 1
		}
		files, err := os.ReadDir(outdir)
		if err != nil {
			return errors.Wrapf(err, "failed to read dir: %s", outdir)
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
				return errors.Wrapf(err, "failed to remove file: %s", fpath)
			}
		}
	} else {
		// create output dir
		err = os.MkdirAll(outdir, 0700)
		if err != nil {
			return errors.Wrapf(err, "failed to create output dir: %s", outdir)
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
	if err := prepareOutpuDir(outputProtoDir, gen.InputOpt.ImportedProtoFiles, true); err != nil {
		return errors.Wrapf(err, "failed to prepare output dir: %s", outputProtoDir)
	}
	if len(gen.InputOpt.Subdirs) != 0 {
		for _, subdir := range gen.InputOpt.Subdirs {
			dir := filepath.Join(gen.InputDir, subdir)
			if err := gen.generate(dir); err != nil {
				return errors.WithMessagef(err, "failed to generate %s", dir)
			}
		}
		return nil
	}
	return gen.generate(gen.InputDir)
}

func (gen *Generator) GenWorkbook(relWorkbookPaths ...string) error {
	outputProtoDir := filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir)
	if err := prepareOutpuDir(outputProtoDir, gen.InputOpt.ImportedProtoFiles, false); err != nil {
		return errors.Wrapf(err, "failed to prepare output dir: %s", outputProtoDir)
	}
	var eg errgroup.Group
	for _, relWorkbookPath := range relWorkbookPaths {
		absPath := filepath.Join(gen.InputDir, relWorkbookPath)
		eg.Go(func() error {
			return gen.convert(filepath.Dir(absPath), filepath.Base(absPath), false)
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
		return errors.Wrapf(err, "failed to read input dir: %s", gen.InputDir)
	}

	// book name -> existence(bool)
	csvBooks := map[string]bool{}
	for _, entry := range dirEntries {
		if entry.IsDir() {
			// scan and generate subdir recursively
			subdir := filepath.Join(dir, entry.Name())
			err = gen.generate(subdir)
			if err != nil {
				return errors.WithMessagef(err, "failed to generate subdir: %s", subdir)
			}
			continue
		} else if gen.InputOpt.FollowSymlink && entry.Type() == iofs.ModeSymlink {
			dstPath, err := os.Readlink(filepath.Join(dir, entry.Name()))
			if err != nil {
				return errors.Wrapf(err, "failed to read symlink: %s", filepath.Join(dir, entry.Name()))
			}
			fileInfo, err := os.Stat(dstPath)
			if err != nil {
				return errors.Wrapf(err, "failed to stat symlink: %s", dstPath)
			}

			if !fileInfo.IsDir() {
				// is not a directory
				log.Warnf("symlink: %s is not a directory, currently not processed", dstPath)
			}
			err = gen.generate(dstPath)
			if err != nil {
				return errors.WithMessagef(err, "failed to generate subdir: %s", dstPath)
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
				return errors.WithMessagef(err, "failed to parse book name from entiry: %s", entry.Name())
			}
			if _, ok := csvBooks[bookName]; ok {
				// NOTE: multiple CSV files construct the same book.
				continue
			}
			csvBooks[bookName] = true
		}

		filename := entry.Name()
		eg.Go(func() error {
			return gen.convert(dir, filename, true)
		})
	}
	return nil
}

func getRelCleanSlashPath(rootdir, dir, filename string) (string, error) {
	relativeDir, err := filepath.Rel(rootdir, dir)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get relative path from %s to %s", rootdir, dir)
	}
	// relative slash separated path
	relativePath := filepath.Join(relativeDir, filename)
	relSlashPath := filepath.ToSlash(filepath.Clean(relativePath))
	return relSlashPath, nil
}

// mergeHeaderOptions merge from options.HeaderOption to tableaupb.SheetMeta.
func mergeHeaderOptions(sheetMeta *tableaupb.SheetMeta, headerOpt *options.HeaderOption) {
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

func (gen *Generator) convert(dir, filename string, checkProtoFileConflicts bool) (err error) {
	absPath := filepath.Join(dir, filename)
	parser := confgen.NewSheetParser(TableauProtoPackage, gen.LocationName, book.MetasheetOptions())
	imp, err := importer.New(absPath, importer.Parser(parser), importer.TopN(defaultTopN))
	if err != nil {
		return errors.Wrapf(err, "failed to import workbook: %s", absPath)
	}

	sheets := imp.GetSheets()
	if len(sheets) == 0 {
		return nil
	}
	basename := filepath.Base(imp.Filename())
	relativePath, err := getRelCleanSlashPath(gen.InputDir, dir, basename)
	if err != nil {
		return errors.WithMessagef(err, "get relative path failed")
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
				// Loader options:
				OrderedMap: sheet.Meta.OrderedMap,
				Index:      sheet.Meta.Index,
			},
			Fields: []*tableaupb.Field{},
			Name:   sheetMsgName,
		}
		shHeader := &sheetHeader{
			meta:    sheet.Meta,
			namerow: sheet.Rows[gen.InputOpt.Header.Namerow-1],
			typerow: sheet.Rows[gen.InputOpt.Header.Typerow-1],
			noterow: sheet.Rows[gen.InputOpt.Header.Noterow-1],
		}

		var parsed bool
		for cursor := 0; cursor < len(shHeader.namerow); cursor++ {
			field := &tableaupb.Field{}
			cursor, parsed, err = bp.parseField(field, shHeader, cursor, "", parseroptions.Nested(sheet.Meta.Nested))
			if err != nil {
				return errors.WithMessagef(err, "failed to parse field")
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
		return errors.WithMessagef(err, "failed to export workbook: %s", relativePath)
	}

	return nil
}

type sheetHeader struct {
	meta    *tableaupb.SheetMeta
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
