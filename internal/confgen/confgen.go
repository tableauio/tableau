package confgen

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Generator struct {
	ProtoPackage string // protobuf package name.
	InputDir     string // input dir of workbooks.
	OutputDir    string // output dir of generated files.

	LocationName string                    // TZ location name.
	InputOpt     *options.ConfInputOption  // Input settings.
	OutputOpt    *options.ConfOutputOption // output settings.

	// Performance stats
	PerfStats sync.Map
}

func NewGenerator(protoPackage, indir, outdir string, setters ...options.Option) *Generator {
	opts := options.ParseOptions(setters...)
	return NewGeneratorWithOptions(protoPackage, indir, outdir, opts)
}

func NewGeneratorWithOptions(protoPackage, indir, outdir string, opts *options.Options) *Generator {
	// TODO: define tableau in package constants.
	if protoPackage == "tableau" {
		log.Panicf(`proto package can not be "tableau" which is reserved`)
	}
	g := &Generator{
		ProtoPackage: protoPackage,
		InputDir:     indir,
		OutputDir:    outdir,
		LocationName: opts.LocationName,
		InputOpt:     opts.Conf.Input,
		OutputOpt:    opts.Conf.Output,
		PerfStats:    sync.Map{},
	}
	return g
}

// bookSpecifier can be:
//   - only workbook: excel/Item.xlsx
//   - specific worksheet: excel/Item.xlsx#Item (To be implemented)
func (gen *Generator) Generate(bookSpecifiers ...string) (err error) {
	defer PrintPerfStats(gen)

	if len(bookSpecifiers) == 0 {
		return gen.GenAll()
	}
	return gen.GenWorkbook(bookSpecifiers...)
}

func (gen *Generator) GenAll() error {
	prFiles, err := loadProtoRegistryFiles(gen.ProtoPackage, gen.InputOpt.ProtoPaths, gen.InputOpt.ProtoFiles, gen.InputOpt.ExcludedProtoFiles...)
	if err != nil {
		return err
	}
	log.Debugf("count of proto files with package name '%s': %v", gen.ProtoPackage, prFiles.NumFilesByPackage(protoreflect.FullName(gen.ProtoPackage)))
	var eg errgroup.Group
	prFiles.RangeFilesByPackage(
		protoreflect.FullName(gen.ProtoPackage),
		func(fd protoreflect.FileDescriptor) bool {
			eg.Go(func() error {
				return gen.convert(prFiles, fd, "")
			})
			return true
		})
	return eg.Wait()
}

// bookSpecifier can be:
//   - only workbook: excel/Item.xlsx
//   - with worksheet: excel/Item.xlsx#Item (To be implemented)
func (gen *Generator) GenWorkbook(bookSpecifiers ...string) error {
	prFiles, err := loadProtoRegistryFiles(gen.ProtoPackage, gen.InputOpt.ProtoPaths, gen.InputOpt.ProtoFiles, gen.InputOpt.ExcludedProtoFiles...)
	if err != nil {
		return err
	}
	log.Debugf("count of proto files with package name %v is %v", gen.ProtoPackage, prFiles.NumFilesByPackage(protoreflect.FullName(gen.ProtoPackage)))
	bookIndexes, err := buildWorkbookIndex(protoreflect.FullName(gen.ProtoPackage), gen.InputDir, gen.InputOpt.SubdirRewrites, prFiles)
	if err != nil {
		return xerrors.WithMessageKV(err, xerrors.KeyModule, xerrors.ModuleConf)
	}
	var eg errgroup.Group
	for _, specifier := range bookSpecifiers {
		bookName, sheetName, err := parseBookSpecifier(specifier)
		if err != nil {
			return errors.Wrapf(err, "parse book specifier failed: %s", specifier)
		}
		relCleanSlashPath := fs.CleanSlashPath(bookName)
		log.Debugf("convert relWorkbookPath to relCleanSlashPath: %s -> %s", bookName, relCleanSlashPath)
		primaryBookIndexInfo, ok := bookIndexes[relCleanSlashPath]
		if !ok {
			if gen.InputOpt.IgnoreUnknownWorkbook {
				log.Debugf("primary workbook not found: %s, but IgnoreUnknownWorkbook is true, so just continue...", relCleanSlashPath)
				continue
			}
			return errors.Errorf("primary workbook not found: %s, protoPaths: %v", relCleanSlashPath, gen.InputOpt.ProtoPaths)
		}
		// NOTE: one book maybe relates to multiple primary books
		for _, fd := range primaryBookIndexInfo.books {
			fd := fd
			eg.Go(func() error {
				return gen.convert(prFiles, fd, sheetName)
			})
		}
	}
	return eg.Wait()
}

// convert a workbook related to parameter fd, and only convert the
// specified worksheet if the input parameter worksheetName is not empty.
func (gen *Generator) convert(prFiles *protoregistry.Files, fd protoreflect.FileDescriptor, worksheetName string) (err error) {
	bookBeginTime := time.Now()
	_, workbook := ParseFileOptions(fd)
	if workbook == nil {
		return nil
	}

	workbookFormat := format.Ext2Format(filepath.Ext(workbook.Name))
	// check if this workbook format need to be converted
	if !format.FilterInput(workbookFormat, gen.InputOpt.Formats) {
		return nil
	}

	// filter subdir
	if !fs.HasSubdirPrefix(workbook.Name, gen.InputOpt.Subdirs) {
		return nil
	}

	// rewrite subdir
	rewrittenWorkbookName := fs.RewriteSubdir(workbook.Name, gen.InputOpt.SubdirRewrites)
	absWbPath := filepath.Join(gen.InputDir, rewrittenWorkbookName)
	log.Debugf("proto: %s, workbook options: %s", fd.Path(), workbook)

	var sheets []string
	// sheet name -> message name
	sheetMap := map[string]*SheetInfo{}
	msgs := fd.Messages()
	for i := 0; i < msgs.Len(); i++ {
		md := msgs.Get(i)
		opts := md.Options().(*descriptorpb.MessageOptions)
		sheetOpts := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
		if sheetOpts != nil {
			sheetMap[sheetOpts.Name] = &SheetInfo{
				ProtoPackage:    gen.ProtoPackage,
				LocationName:    gen.LocationName,
				PrimaryBookName: rewrittenWorkbookName,
				MD:              md,
				Opts:            sheetOpts,
				ExtInfo: &SheetParserExtInfo{
					InputDir:       gen.InputDir,
					SubdirRewrites: gen.InputOpt.SubdirRewrites,
					PRFiles:        prFiles,
				},
			}
			sheets = append(sheets, sheetOpts.Name)
		}
	}

	imp, err := importer.New(absWbPath, importer.Sheets(sheets), importer.Mode(importer.Confgen))
	if err != nil {
		return xerrors.WithMessageKV(err, xerrors.KeyModule, xerrors.ModuleConf, xerrors.KeyBookName, workbook.Name)
	}
	bookPrepareMilliseconds := time.Since(bookBeginTime).Milliseconds()
	worksheetFound := false
	for sheetName, sheetInfo := range sheetMap {
		sheetBeginTime := time.Now()
		if worksheetName != "" {
			if worksheetName != sheetName {
				continue
			}
			worksheetFound = true
		}
		// log.Debugf("%s", md.FullName())
		log.Infof("%18s: %s#%s (%s#%s)", "parsing worksheet", fd.Path(), sheetInfo.MD.Name(), workbook.Name, sheetName)

		if sheetInfo.HasScatter() {
			if sheetInfo.HasMerger() {
				return xerrors.ErrorKV("option Scatter and Merger cannot be both set at one sheet",
					xerrors.KeyModule, xerrors.ModuleConf, xerrors.KeyBookName, workbook.Name, xerrors.KeySheetName, worksheetName)
			}
			err := gen.processScatter(imp, sheetInfo, rewrittenWorkbookName, sheetName)
			if err != nil {
				return xerrors.WithMessageKV(err, xerrors.KeyModule, xerrors.ModuleConf, xerrors.KeyBookName, workbook.Name, xerrors.KeySheetName, worksheetName)
			}
		} else {
			err := gen.processMerger(imp, sheetInfo, rewrittenWorkbookName, sheetName)
			if err != nil {
				return xerrors.WithMessageKV(err, xerrors.KeyModule, xerrors.ModuleConf, xerrors.KeyBookName, workbook.Name, xerrors.KeySheetName, worksheetName)
			}
		}

		seconds := time.Since(sheetBeginTime).Milliseconds() + bookPrepareMilliseconds
		gen.PerfStats.Store(sheetInfo.MD.Name(), seconds)
	}
	if worksheetName != "" && !worksheetFound {
		return xerrors.ErrorKV(fmt.Sprintf("worksheet not found: %s", worksheetName),
			xerrors.KeyModule, xerrors.ModuleConf,
			xerrors.KeyBookName, workbook.Name,
			xerrors.KeySheetName, worksheetName)
	}
	return nil
}

func (gen *Generator) processScatter(self importer.Importer, sheetInfo *SheetInfo, workbookName, sheetName string) error {
	importers, err := importer.GetScatterImporters(gen.InputDir, workbookName, sheetName, sheetInfo.Opts.Scatter, gen.InputOpt.SubdirRewrites)
	if err != nil {
		return err
	}
	// append self
	importers = append(importers, importer.ImporterInfo{Importer: self})
	exporter := NewSheetExporter(gen.OutputDir, gen.OutputOpt)
	if err := exporter.ScatterAndExport(sheetInfo, importers...); err != nil {
		return err
	}
	return nil
}

func (gen *Generator) processMerger(self importer.Importer, sheetInfo *SheetInfo, workbookName, sheetName string) error {
	importers, err := importer.GetMergerImporters(gen.InputDir, workbookName, sheetName, sheetInfo.Opts.Merger, gen.InputOpt.SubdirRewrites)
	if err != nil {
		return err
	}
	// append self
	importers = append(importers, importer.ImporterInfo{Importer: self})
	exporter := NewSheetExporter(gen.OutputDir, gen.OutputOpt)
	if err := exporter.MergeAndExport(sheetInfo, importers...); err != nil {
		return err
	}
	return nil
}
