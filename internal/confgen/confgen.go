package confgen

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"

	"buf.build/go/protovalidate"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/confgen/fieldprop"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/metasheet"
	"github.com/tableauio/tableau/internal/strcase"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"

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

	LocationName  string                    // TZ location name.
	InputOpt      *options.ConfInputOption  // Input settings.
	OutputOpt     *options.ConfOutputOption // output settings.
	ErrorLimitOpt *options.ErrorLimitOption // error collection limits.

	enableProfiling bool
	validator       protovalidate.Validator // validator with extension type resolver for custom predefined rules.
	collector       *xerrors.Collector
	referredCache   *fieldprop.ReferredCache

	// Performance stats
	SheetParserStats sync.Map
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

	errorLimit := opts.ErrorLimit
	if errorLimit == nil {
		errorLimit = &options.ErrorLimitOption{
			MaxErrors:         options.DefaultMaxErrors,
			MaxErrorsPerBook:  options.DefaultMaxErrorsPerBook,
			MaxErrorsPerSheet: options.DefaultMaxErrorsPerSheet,
		}
	}

	g := &Generator{
		ProtoPackage:     protoPackage,
		InputDir:         indir,
		OutputDir:        outdir,
		LocationName:     opts.LocationName,
		InputOpt:         opts.Conf.Input,
		OutputOpt:        opts.Conf.Output,
		ErrorLimitOpt:    errorLimit,
		enableProfiling:  opts.Profiling,
		ctx:              ctx,
		collector:        xerrors.NewCollector(errorLimit.MaxErrors),
		referredCache:    fieldprop.NewReferredCache(),
		SheetParserStats: sync.Map{},
	}
	return g
}

func (gen *Generator) resetRunState() {
	gen.collector = xerrors.NewCollector(gen.ErrorLimitOpt.MaxErrors)
	gen.referredCache = fieldprop.NewReferredCache()
	gen.SheetParserStats = sync.Map{}
}

// bookSpecifier can be:
//   - only workbook: excel/Item.xlsx
//   - specific worksheet: excel/Item.xlsx#Item (To be implemented)
func (gen *Generator) Generate(bookSpecifiers ...string) (err error) {
	if len(bookSpecifiers) == 0 {
		return gen.GenAll()
	}
	return gen.GenWorkbook(bookSpecifiers...)
}

func (gen *Generator) GenAll() (err error) {
	gen.resetRunState()
	defer PrintPerfStats(gen)
	stopProfiling, err := gen.startProfiling()
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, stopProfiling())
	}()
	prFiles, err := loadProtoRegistryFiles(gen.ProtoPackage, gen.InputOpt.ProtoPaths, gen.InputOpt.ProtoFiles, gen.InputOpt.ExcludedProtoFiles...)
	if err != nil {
		return err
	}
	// Create a validator with extension type resolver for custom predefined rules.
	gen.validator, err = NewValidator(prFiles)
	if err != nil {
		return err
	}
	log.Debugf("count of proto files with package name '%s': %v", gen.ProtoPackage, prFiles.NumFilesByPackage(protoreflect.FullName(gen.ProtoPackage)))
	g := gen.collector.NewGroup(context.Background())
	prFiles.RangeFilesByPackage(
		protoreflect.FullName(gen.ProtoPackage),
		func(fd protoreflect.FileDescriptor) bool {
			g.Go(func(ctx context.Context) error {
				return gen.convert(prFiles, fd, "")
			})
			return true
		})
	return g.Wait()
}

// bookSpecifier can be:
//   - only workbook: excel/Item.xlsx
//   - with worksheet: excel/Item.xlsx#Item (To be implemented)
func (gen *Generator) GenWorkbook(bookSpecifiers ...string) (err error) {
	gen.resetRunState()
	defer PrintPerfStats(gen)
	stopProfiling, err := gen.startProfiling()
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, stopProfiling())
	}()
	prFiles, err := loadProtoRegistryFiles(gen.ProtoPackage, gen.InputOpt.ProtoPaths, gen.InputOpt.ProtoFiles, gen.InputOpt.ExcludedProtoFiles...)
	if err != nil {
		return err
	}
	// Create a validator with extension type resolver for custom predefined rules.
	gen.validator, err = NewValidator(prFiles)
	if err != nil {
		return err
	}
	log.Debugf("count of proto files with package name %v is %v", gen.ProtoPackage, prFiles.NumFilesByPackage(protoreflect.FullName(gen.ProtoPackage)))
	bookIndexes, err := buildWorkbookIndex(gen.ProtoPackage, gen.InputDir, gen.InputOpt.Subdirs, gen.InputOpt.SubdirRewrites, prFiles)
	if err != nil {
		return xerrors.WrapKV(err, xerrors.KeyModule, xerrors.ModuleConf)
	}
	g := gen.collector.NewGroup(context.Background())
	for _, specifier := range bookSpecifiers {
		bookName, sheetName, err := parseBookSpecifier(specifier)
		if err != nil {
			return xerrors.Wrapf(err, "parse book specifier failed: %s", specifier)
		}
		relCleanSlashPath := xfs.CleanSlashPath(bookName)
		log.Debugf("convert relWorkbookPath to relCleanSlashPath: %s -> %s", bookName, relCleanSlashPath)
		primaryBookInfo, ok := bookIndexes.get(relCleanSlashPath)
		if !ok {
			if gen.InputOpt.IgnoreUnknownWorkbook {
				log.Debugf("primary workbook not found: %s, but IgnoreUnknownWorkbook is true, so just continue...", relCleanSlashPath)
				continue
			}
			return xerrors.Newf("primary workbook not found: %s, protoPaths: %v", relCleanSlashPath, gen.InputOpt.ProtoPaths)
		}
		// NOTE: one book may relate to multiple primary books
		for _, fd := range primaryBookInfo.fds {
			g.Go(func(ctx context.Context) error {
				return gen.convert(prFiles, fd, sheetName)
			})
		}
	}
	return g.Wait()
}

// convert a workbook related to parameter fd, and only convert the
// specified worksheet if the input parameter worksheetName is not empty.
func (gen *Generator) convert(prFiles *protoregistry.Files, fd protoreflect.FileDescriptor, specifiedSheetName string) (err error) {
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
				ErrorLimit:     gen.ErrorLimitOpt,
				ReferredCache:  gen.referredCache,
				SheetParserStats: func() *sync.Map {
					if gen.enableProfiling {
						return &gen.SheetParserStats
					}
					return nil
				}(),
			},
		})
		// NOTE: one sheet may be generated to multiple messages (e.g.: full version and lite version) in the same workbook.
		if !slices.Contains(sheetNames, sheetOpts.Name) {
			sheetNames = append(sheetNames, sheetOpts.Name)
		}
	}

	// Skip workbook protos with no worksheet messages (e.g. union shards).
	if len(sheets) == 0 {
		return nil
	}

	imp, err := importer.New(gen.ctx, absWbPath, importer.Sheets(sheetNames), importer.Mode(importer.Confgen))
	if err != nil {
		return xerrors.WrapKV(err, xerrors.KeyModule, xerrors.ModuleConf, xerrors.KeyBookName, workbook.Name)
	}
	bookCollector := gen.collector.NewChild(gen.ErrorLimitOpt.MaxErrorsPerBook,
		xerrors.KeyModule, xerrors.ModuleConf,
		xerrors.KeyBookName, workbook.Name,
		xerrors.KeyPrimaryBookName, rewrittenWorkbookName)
	worksheetFound := false
	for _, sheetInfo := range sheets {
		sheetName := sheetInfo.SheetName()
		if specifiedSheetName != "" {
			if specifiedSheetName != sheetName {
				continue
			}
			worksheetFound = true
		}
		// log.Debugf("%s", md.FullName())
		log.Infof("%15s: %s#%s (%s#%s)", "parsing sheet", fd.Path(), sheetInfo.MD.Name(), workbook.Name, sheetName)
		messageCollector := bookCollector.NewChild(0,
			xerrors.KeySheetName, sheetName,
			xerrors.KeyPrimarySheetName, sheetName,
			xerrors.KeyPBMessage, string(sheetInfo.MD.Name()))

		if sheetInfo.HasScatter() {
			if sheetInfo.HasMerger() {
				return xerrors.NewKV("option Scatter and Merger cannot be both set at one sheet",
					xerrors.KeyModule, xerrors.ModuleConf, xerrors.KeyBookName, workbook.Name, xerrors.KeySheetName, sheetName)
			}
			if err := gen.processScatter(imp, sheetInfo, messageCollector); err != nil {
				if err := messageCollector.Collect(err); err != nil {
					return err
				}
				continue
			}
		} else {
			if err := gen.processMerger(imp, sheetInfo, messageCollector); err != nil {
				if err := messageCollector.Collect(err); err != nil {
					return err
				}
				continue
			}
		}

	}
	if specifiedSheetName != "" && !worksheetFound {
		return xerrors.NewKV(fmt.Sprintf("worksheet not found: %s", specifiedSheetName),
			xerrors.KeyModule, xerrors.ModuleConf,
			xerrors.KeyBookName, workbook.Name,
			xerrors.KeySheetName, specifiedSheetName)
	}
	if bookCollector.HasErrors() {
		return bookCollector.Join()
	}
	return nil
}

func (gen *Generator) processScatter(self importer.Importer, sheetInfo *SheetInfo, messageCollector *xerrors.Collector) error {
	importers, err := importer.GetScatterImporters(gen.ctx, gen.InputDir, sheetInfo.BookName(), sheetInfo.SheetName(), sheetInfo.SheetOpts.Scatter, gen.InputOpt.SubdirRewrites)
	if err != nil {
		return err
	}
	mainImporter := importer.ImporterInfo{Importer: self}
	exporter := NewSheetExporter(gen.OutputDir, gen.OutputOpt, gen.validator, messageCollector)
	if err := exporter.ScatterAndExport(sheetInfo, mainImporter, importers...); err != nil {
		return err
	}
	return nil
}

func (gen *Generator) processMerger(self importer.Importer, sheetInfo *SheetInfo, messageCollector *xerrors.Collector) error {
	importers, err := importer.GetMergerImporters(gen.ctx, gen.InputDir, sheetInfo.BookName(), sheetInfo.SheetName(), sheetInfo.SheetOpts.Merger, gen.InputOpt.SubdirRewrites)
	if err != nil {
		return err
	}
	mainImporter := importer.ImporterInfo{Importer: self}
	exporter := NewSheetExporter(gen.OutputDir, gen.OutputOpt, gen.validator, messageCollector)
	if err := exporter.MergeAndExport(sheetInfo, mainImporter, importers...); err != nil {
		return err
	}
	return nil
}
