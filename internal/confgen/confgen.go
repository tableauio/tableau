package confgen

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/xproto"
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

	// protoregistry
	prFiles *protoregistry.Files

	// Performace stats
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

func (gen *Generator) Generate(relWorkbookPaths ...string) (err error) {
	defer PrintPerfStats(gen)

	if len(relWorkbookPaths) == 0 {
		return gen.GenAll()
	}
	return gen.GenWorkbook(relWorkbookPaths...)
}

func (gen *Generator) GenAll() error {
	// create output dir
	outputConfDir := filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir)
	err := os.MkdirAll(outputConfDir, 0700)
	if err != nil {
		return xerrors.WrapKV(err, "OutputDir", outputConfDir)
	}

	err = gen.loadProtoRegistryFiles(gen.ProtoPackage, gen.InputOpt.ProtoPaths, gen.InputOpt.ProtoFiles, gen.InputOpt.ExcludedProtoFiles...)
	if err != nil {
		return err
	}

	log.Debugf("count of proto files with package name '%s': %v", gen.ProtoPackage, gen.prFiles.NumFilesByPackage(protoreflect.FullName(gen.ProtoPackage)))

	var eg errgroup.Group
	gen.prFiles.RangeFilesByPackage(
		protoreflect.FullName(gen.ProtoPackage),
		func(fd protoreflect.FileDescriptor) bool {
			eg.Go(func() error {
				return gen.convert(fd, "")
			})
			return true
		})
	return eg.Wait()
}

func (gen *Generator) GenWorkbook(relWorkbookPaths ...string) error {
	var eg errgroup.Group
	for _, relWorkbookPath := range relWorkbookPaths {
		tempRelWorkbookPath := relWorkbookPath
		eg.Go(func() error {
			return gen.GenOneWorkbook(tempRelWorkbookPath, "")
		})
	}
	return eg.Wait()
}

func (gen *Generator) GenOneWorkbook(relWorkbookPath string, worksheetName string) (err error) {
	relCleanSlashPath := fs.GetCleanSlashPath(relWorkbookPath)
	if err != nil {
		return errors.Wrapf(err, "failed to get relative path from %s to %s", gen.InputDir, relWorkbookPath)
	}
	log.Debugf("convert relWorkbookPath to relCleanSlashPath: %s -> %s", relWorkbookPath, relCleanSlashPath)
	relWorkbookPath = relCleanSlashPath

	// create output dir
	outputConfDir := filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir)
	err = os.MkdirAll(outputConfDir, 0700)
	if err != nil {
		return errors.WithMessagef(err, "failed to create output dir: %s", outputConfDir)
	}
	err = gen.loadProtoRegistryFiles(gen.ProtoPackage, gen.InputOpt.ProtoPaths, gen.InputOpt.ProtoFiles, gen.InputOpt.ExcludedProtoFiles...)
	if err != nil {
		return errors.WithMessagef(err, "failed to create files")
	}
	log.Debugf("count of proto files with package name %v is %v", gen.ProtoPackage, gen.prFiles.NumFilesByPackage(protoreflect.FullName(gen.ProtoPackage)))

	workbookFound := false
	gen.prFiles.RangeFilesByPackage(
		protoreflect.FullName(gen.ProtoPackage),
		func(fd protoreflect.FileDescriptor) bool {
			_, workbook := ParseFileOptions(fd)
			if workbook == nil {
				return true
			}
			// rewrite subdir
			rewrittenWorkbookName := fs.RewriteSubdir(workbook.Name, gen.InputOpt.SubdirRewrites)
			if relWorkbookPath != "" && relWorkbookPath != rewrittenWorkbookName {
				return true
			}
			workbookFound = true
			// Due to closure, this err will be returned by func Generate().
			err = gen.convert(fd, worksheetName)
			return false
		})

	if err != nil {
		return err
	}
	if !workbookFound {
		if relWorkbookPath == "" {
			return errors.Errorf("There's no any workbook found, maybe you forget to use `blank identifier` to inject the protoconf package.")
		}
		return errors.Errorf("workbook not found: %s, protoPaths: %v", relWorkbookPath, gen.InputOpt.ProtoPaths)
	}
	return nil
}

// convert a workbook related to parameter fd, and only convert the
// specified worksheet if the input parameter worksheetName is not empty.
func (gen *Generator) convert(fd protoreflect.FileDescriptor, worksheetName string) (err error) {
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
	if !fs.FilterSubdir(workbook.Name, gen.InputOpt.Subdirs) {
		return nil
	}

	var sheets []string
	// sheet name -> message name
	sheetMap := map[string]SheetInfo{}
	msgs := fd.Messages()
	for i := 0; i < msgs.Len(); i++ {
		md := msgs.Get(i)
		opts := md.Options().(*descriptorpb.MessageOptions)
		sheetOpts := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
		if sheetOpts != nil {
			sheetMap[sheetOpts.Name] = SheetInfo{
				ProtoPackage: gen.ProtoPackage,
				LocationName: gen.LocationName,
				MD:           md,
				Opts:         sheetOpts,
				gen:          gen,
			}
			sheets = append(sheets, sheetOpts.Name)
		}
	}

	// rewrite subdir
	rewrittenWorkbookName := fs.RewriteSubdir(workbook.Name, gen.InputOpt.SubdirRewrites)
	wbPath := filepath.Join(gen.InputDir, rewrittenWorkbookName)
	log.Debugf("proto: %s, workbook options: %s", fd.Path(), workbook)
	imp, err := importer.New(wbPath, importer.Sheets(sheets), importer.Mode(importer.Confgen))
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
			err := gen.processScatter(imp, &sheetInfo, wbPath, sheetName)
			if err != nil {
				return xerrors.WithMessageKV(err, xerrors.KeyModule, xerrors.ModuleConf, xerrors.KeyBookName, workbook.Name, xerrors.KeySheetName, worksheetName)
			}
		} else {
			err := gen.processMerger(imp, &sheetInfo, wbPath, sheetName)
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

func (gen *Generator) processScatter(self importer.Importer, sheetInfo *SheetInfo, wbPath, sheetName string) error {
	importers, err := importer.GetScatterImporters(gen.InputDir, wbPath, sheetName, sheetInfo.Opts.Scatter)
	if err != nil {
		return err
	}
	// append self
	importers = append(importers, self)
	exporter := NewSheetExporter(gen.OutputDir, gen.OutputOpt)
	if err := exporter.ScatterAndExport(sheetInfo, importers...); err != nil {
		return err
	}
	return nil
}

func (gen *Generator) processMerger(self importer.Importer, sheetInfo *SheetInfo, wbPath, sheetName string) error {
	importers, err := importer.GetMergerImporters(gen.InputDir, wbPath, sheetName, sheetInfo.Opts.Merger)
	if err != nil {
		return err
	}
	// append self
	importers = append(importers, self)
	exporter := NewSheetExporter(gen.OutputDir, gen.OutputOpt)
	if err := exporter.MergeAndExport(sheetInfo, importers...); err != nil {
		return err
	}
	return nil
}

// get protoregistry.Files with specified package name
func (gen *Generator) loadProtoRegistryFiles(protoPackage string, protoPaths []string, protoFiles []string, excludeProtoFiles ...string) error {
	count := 0
	prFiles := protoregistry.GlobalFiles
	prFiles.RangeFilesByPackage(
		protoreflect.FullName(protoPackage),
		func(fd protoreflect.FileDescriptor) bool {
			count++
			return false
		})
	if count != 0 {
		log.Debugf("use already injected protoregistry.GlobalFiles")
		gen.prFiles = prFiles
		return nil
	}
	prFiles, err := xproto.NewFiles(protoPaths, protoFiles, excludeProtoFiles...)
	if err != nil {
		return err
	}
	gen.prFiles = prFiles
	return nil
}
