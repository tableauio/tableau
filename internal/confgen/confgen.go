package confgen

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/metasheet"
	"github.com/tableauio/tableau/internal/strcase"
	"github.com/tableauio/tableau/internal/x/xfs"
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
	ctx          context.Context
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
	ctx := context.Background()
	ctx = strcase.NewContext(ctx, strcase.New(opts.Acronyms))
	metasheetName := metasheet.DefaultMetasheetName
	// use the metasheet name from the proto input settings if provided.
	if opts.Proto != nil && opts.Proto.Input != nil {
		metasheetName = opts.Proto.Input.MetasheetName
	}
	ctx = metasheet.NewContext(ctx, &metasheet.Metasheet{Name: metasheetName})

	g := &Generator{
		ProtoPackage: protoPackage,
		InputDir:     indir,
		OutputDir:    outdir,
		LocationName: opts.LocationName,
		InputOpt:     opts.Conf.Input,
		OutputOpt:    opts.Conf.Output,
		ctx:          ctx,
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
	bookIndexes, err := buildWorkbookIndex(gen.ProtoPackage, gen.InputDir, gen.InputOpt.Subdirs, gen.InputOpt.SubdirRewrites, prFiles)
	if err != nil {
		return xerrors.WrapKV(err, xerrors.KeyModule, xerrors.ModuleConf)
	}
	var eg errgroup.Group
	for _, specifier := range bookSpecifiers {
		bookName, sheetName, err := parseBookSpecifier(specifier)
		if err != nil {
			return xerrors.Wrapf(err, "parse book specifier failed: %s", specifier)
		}
		relCleanSlashPath := xfs.CleanSlashPath(bookName)
		log.Debugf("convert relWorkbookPath to relCleanSlashPath: %s -> %s", bookName, relCleanSlashPath)
		primaryBookInfo, ok := bookIndexes[relCleanSlashPath]
		if !ok {
			if gen.InputOpt.IgnoreUnknownWorkbook {
				log.Debugf("primary workbook not found: %s, but IgnoreUnknownWorkbook is true, so just continue...", relCleanSlashPath)
				continue
			}
			return xerrors.Errorf("primary workbook not found: %s, protoPaths: %v", relCleanSlashPath, gen.InputOpt.ProtoPaths)
		}
		// NOTE: one book may relate to multiple primary books
		for _, fd := range primaryBookInfo.fds {
			fd := fd // TODO: go1.22 fixes loopvar problem
			eg.Go(func() error {
				return gen.convert(prFiles, fd, sheetName)
			})
		}
	}
	return eg.Wait()
}

// convert a workbook related to parameter fd, and only convert the
// specified worksheet if the input parameter worksheetName is not empty.
func (gen *Generator) convert(prFiles *protoregistry.Files, fd protoreflect.FileDescriptor, specifiedSheetName string) (err error) {
	bookBeginTime := time.Now()
	_, workbook := ParseFileOptions(fd)
	if workbook == nil {
		return nil
	}

	workbookFormat := format.GetFormat(workbook.Name)
	// check if this workbook format need to be converted
	if !format.FilterInput(workbookFormat, gen.InputOpt.Formats) {
		return nil
	}

	// filter subdir
	if !xfs.HasSubdirPrefix(workbook.Name, gen.InputOpt.Subdirs) {
		return nil
	}

	// rewrite subdir
	rewrittenWorkbookName := xfs.RewriteSubdir(workbook.Name, gen.InputOpt.SubdirRewrites)
	absWbPath := filepath.Join(gen.InputDir, rewrittenWorkbookName)
	log.Debugf("proto: %s, workbook options: %s", fd.Path(), workbook)

	var sheetNames []string
	var sheets []*SheetInfo
	fileOpts := fd.Options().(*descriptorpb.FileOptions)
	bookOpts := proto.GetExtension(fileOpts, tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
	msgs := fd.Messages()
	for i := 0; i < msgs.Len(); i++ {
		md := msgs.Get(i)
		opts := md.Options().(*descriptorpb.MessageOptions)
		sheetOpts := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
		if sheetOpts == nil {
			continue // skip non-sheet
		}
		sheets = append(sheets, &SheetInfo{
			ProtoPackage:    gen.ProtoPackage,
			LocationName:    gen.LocationName,
			PrimaryBookName: rewrittenWorkbookName,
			MD:              md,
			BookOpts:        bookOpts,
			SheetOpts:       sheetOpts,
			ExtInfo: &SheetParserExtInfo{
				InputDir:       gen.InputDir,
				SubdirRewrites: gen.InputOpt.SubdirRewrites,
				PRFiles:        prFiles,
				BookFormat:     workbookFormat,
				DryRun:         gen.OutputOpt.DryRun,
			},
		})
		// NOTE: one sheet may be generated to multiple messages (e.g.: full version and lite version) in the same workbook.
		if !slices.Contains(sheetNames, sheetOpts.Name) {
			sheetNames = append(sheetNames, sheetOpts.Name)
		}
	}

	imp, err := importer.New(gen.ctx, absWbPath, importer.Sheets(sheetNames), importer.Mode(importer.Confgen))
	if err != nil {
		return xerrors.WrapKV(err, xerrors.KeyModule, xerrors.ModuleConf, xerrors.KeyBookName, workbook.Name)
	}
	bookPrepareMilliseconds := time.Since(bookBeginTime).Milliseconds()
	worksheetFound := false
	for _, sheetInfo := range sheets {
		sheetName := sheetInfo.SheetName()
		sheetBeginTime := time.Now()
		if specifiedSheetName != "" {
			if specifiedSheetName != sheetName {
				continue
			}
			worksheetFound = true
		}
		// log.Debugf("%s", md.FullName())
		log.Infof("%15s: %s#%s (%s#%s)", "parsing sheet", fd.Path(), sheetInfo.MD.Name(), workbook.Name, sheetName)

		if sheetInfo.HasScatter() {
			if sheetInfo.HasMerger() {
				return xerrors.ErrorKV("option Scatter and Merger cannot be both set at one sheet",
					xerrors.KeyModule, xerrors.ModuleConf, xerrors.KeyBookName, workbook.Name, xerrors.KeySheetName, specifiedSheetName)
			}
			err := gen.processScatter(imp, sheetInfo)
			if err != nil {
				return xerrors.WrapKV(err, xerrors.KeyModule, xerrors.ModuleConf, xerrors.KeyBookName, workbook.Name, xerrors.KeySheetName, specifiedSheetName)
			}
		} else {
			err := gen.processMerger(imp, sheetInfo)
			if err != nil {
				return xerrors.WrapKV(err, xerrors.KeyModule, xerrors.ModuleConf, xerrors.KeyBookName, workbook.Name, xerrors.KeySheetName, specifiedSheetName)
			}
		}

		seconds := time.Since(sheetBeginTime).Milliseconds() + bookPrepareMilliseconds
		gen.PerfStats.Store(sheetInfo.MD.Name(), seconds)
	}
	if specifiedSheetName != "" && !worksheetFound {
		return xerrors.ErrorKV(fmt.Sprintf("worksheet not found: %s", specifiedSheetName),
			xerrors.KeyModule, xerrors.ModuleConf,
			xerrors.KeyBookName, workbook.Name,
			xerrors.KeySheetName, specifiedSheetName)
	}
	return nil
}

func (gen *Generator) processScatter(self importer.Importer, sheetInfo *SheetInfo) error {
	importers, err := importer.GetScatterImporters(gen.ctx, gen.InputDir, sheetInfo.BookName(), sheetInfo.SheetName(), sheetInfo.SheetOpts.Scatter, gen.InputOpt.SubdirRewrites)
	if err != nil {
		return err
	}
	mainImporter := importer.ImporterInfo{Importer: self}
	exporter := NewSheetExporter(gen.OutputDir, gen.OutputOpt)
	if err := exporter.ScatterAndExport(sheetInfo, mainImporter, importers...); err != nil {
		return err
	}
	return nil
}

func (gen *Generator) processMerger(self importer.Importer, sheetInfo *SheetInfo) error {
	importers, err := importer.GetMergerImporters(gen.ctx, gen.InputDir, sheetInfo.BookName(), sheetInfo.SheetName(), sheetInfo.SheetOpts.Merger, gen.InputOpt.SubdirRewrites)
	if err != nil {
		return err
	}
	mainImporter := importer.ImporterInfo{Importer: self}
	exporter := NewSheetExporter(gen.OutputDir, gen.OutputOpt)
	if err := exporter.MergeAndExport(sheetInfo, mainImporter, importers...); err != nil {
		return err
	}
	return nil
}
