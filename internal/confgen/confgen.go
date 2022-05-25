package confgen

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/emirpasic/gods/lists/arraylist"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/xproto"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type Generator struct {
	ProtoPackage string // protobuf package name.
	LocationName string // Location represents the collection of time offsets in use in a geographical area. Default is "Asia/Shanghai".
	InputDir     string // input dir of workbooks.
	OutputDir    string // output dir of generated files.

	Header    *options.HeaderOption     // header settings.
	InputOpt  *options.InputConfOption  // Input settings.
	OutputOpt *options.OutputConfOption // output settings.

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
		atom.Log.Panicf(`proto package can not be "tableau" which is reserved`)
	}
	g := &Generator{
		ProtoPackage: protoPackage,
		LocationName: opts.LocationName,
		InputDir:     indir,
		OutputDir:    outdir,
		Header:       opts.Header,
		InputOpt:     opts.Input.Conf,
		OutputOpt:    opts.Output.Conf,
		PerfStats:    sync.Map{},
	}
	return g
}

type sheetInfo struct {
	MessageName string
	opts        *tableaupb.WorksheetOptions
}

type messagerStatsInfo struct {
	Name         string
	Milliseconds int64
}

func PrintPerfStats(gen *Generator) {
	// print performance stats
	list := arraylist.New()
	gen.PerfStats.Range(func(key, value interface{}) bool {
		list.Add(&messagerStatsInfo{
			Name:         key.(string),
			Milliseconds: value.(int64),
		})
		return true
	})
	list.Sort(func(a, b interface{}) int {
		infoA := a.(*messagerStatsInfo)
		infoB := b.(*messagerStatsInfo)
		return int(infoB.Milliseconds - infoA.Milliseconds)
	})
	list.Each(func(index int, value interface{}) {
		info := value.(*messagerStatsInfo)
		atom.Log.Debugf("timespan|%v: %vs", info.Name, float64(info.Milliseconds)/1000)
	})
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
		return errors.WithMessagef(err, "failed to create output dir: %s", outputConfDir)
	}

	prFiles, err := getProtoRegistryFiles(gen.ProtoPackage, gen.InputOpt.ProtoPaths, gen.InputOpt.ProtoFiles...)
	if err != nil {
		return errors.WithMessagef(err, "failed to create files")
	}

	atom.Log.Debugf("count of proto files with package name %s is %s", gen.ProtoPackage, prFiles.NumFilesByPackage(protoreflect.FullName(gen.ProtoPackage)))

	var eg errgroup.Group
	prFiles.RangeFilesByPackage(
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
	atom.Log.Debugf("convert relWorkbookPath to relCleanSlashPath: %s -> %s", relWorkbookPath, relCleanSlashPath)
	relWorkbookPath = relCleanSlashPath

	// create output dir
	outputConfDir := filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir)
	err = os.MkdirAll(outputConfDir, 0700)
	if err != nil {
		return errors.WithMessagef(err, "failed to create output dir: %s", outputConfDir)
	}
	prFiles, err := getProtoRegistryFiles(gen.ProtoPackage, gen.InputOpt.ProtoPaths, gen.InputOpt.ProtoFiles...)
	if err != nil {
		return errors.WithMessagef(err, "failed to create files")
	}
	atom.Log.Debugf("count of proto files with package name %v is %v", gen.ProtoPackage, prFiles.NumFilesByPackage(protoreflect.FullName(gen.ProtoPackage)))

	workbookFound := false
	prFiles.RangeFilesByPackage(
		protoreflect.FullName(gen.ProtoPackage),
		func(fd protoreflect.FileDescriptor) bool {
			_, workbook := ParseFileOptions(fd)
			if workbook == nil {
				return true
			}
			if relWorkbookPath != "" && relWorkbookPath != workbook.Name {
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
		return errors.Errorf("workbook not found: %s", relWorkbookPath)
	}
	return nil
}

// convert a workbook related to parameter fd, and only convert the
// specified worksheet if the input parameter worksheetName is not empty.
func (gen *Generator) convert(fd protoreflect.FileDescriptor, worksheetName string) error {
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
	sheetMap := map[string]sheetInfo{}
	msgs := fd.Messages()
	for i := 0; i < msgs.Len(); i++ {
		md := msgs.Get(i)
		opts := md.Options().(*descriptorpb.MessageOptions)
		worksheet := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
		if worksheet != nil {
			sheetMap[worksheet.Name] = sheetInfo{string(md.Name()), worksheet}
			sheets = append(sheets, worksheet.Name)
		}
	}

	// rewrite subdir
	rewrittenWorkbookName := fs.RewriteSubdir(workbook.Name, gen.InputOpt.SubdirRewrites)
	wbPath := filepath.Join(gen.InputDir, rewrittenWorkbookName)
	atom.Log.Debugf("proto: %s, workbook options: %s", fd.Path(), workbook)
	imp, err := importer.New(wbPath, importer.Sheets(sheets))
	if err != nil {
		return errors.WithMessagef(err, "failed to import workbook: %s", wbPath)
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
		md := msgs.ByName(protoreflect.Name(sheetInfo.MessageName))
		// atom.Log.Debugf("%s", md.FullName())
		atom.Log.Infof("generate: %s#%s (%s#%s)", fd.Path(), md.Name(), workbook.Name, sheetName)
		newMsg := dynamicpb.NewMessage(md)
		parser := NewSheetParser(gen.ProtoPackage, gen.LocationName, sheetInfo.opts)

		// get merger importers
		importers, err := getMergerImporters(wbPath, sheetName, sheetInfo.opts.Merger)
		if err != nil {
			return errors.WithMessagef(err, "failed to get merger importers for %s", wbPath)
		}
		// append self
		importers = append(importers, imp)

		exporter := NewSheetExporter(gen.OutputDir, gen.OutputOpt)
		if err := exporter.Export(parser, newMsg, importers...); err != nil {
			return err
		}
		seconds := time.Since(sheetBeginTime).Milliseconds() + bookPrepareMilliseconds
		gen.PerfStats.Store(sheetInfo.MessageName, seconds)
	}
	if worksheetName != "" && !worksheetFound {
		return errors.Errorf("worksheet not found: %s", worksheetName)
	}
	return nil
}

// getMergerImporters gathers all merger importers.
// 	1. support Glob pattern, refer https://pkg.go.dev/path/filepath#Glob
// 	2. exclude self
func getMergerImporters(primaryWorkbookPath, sheetName string, merger []string) ([]importer.Importer, error) {
	if len(merger) == 0 {
		return nil, nil
	}

	curDir := filepath.Dir(primaryWorkbookPath)
	mergerWorkbookPaths := map[string]bool{}
	for _, merger := range merger {
		pattern := filepath.Join(curDir, merger)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to glob pattern: %s", pattern)
		}
		for _, match := range matches {
			if fs.IsSamePath(match, primaryWorkbookPath) {
				// exclude self
				continue
			}
			mergerWorkbookPaths[match] = true
		}
	}
	var importers []importer.Importer
	for fpath := range mergerWorkbookPaths {
		atom.Log.Infof("merge workbook: %s", fpath)
		importer, err := importer.New(fpath, importer.Sheets([]string{sheetName}))
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to create importer: %s", fpath)
		}
		importers = append(importers, importer)
	}
	return importers, nil
}

// get protoregistry.Files with specified package name
func getProtoRegistryFiles(protoPackage string, importPaths []string, protoFiles ...string) (*protoregistry.Files, error) {
	count := 0
	prFiles := protoregistry.GlobalFiles
	prFiles.RangeFilesByPackage(
		protoreflect.FullName(protoPackage),
		func(fd protoreflect.FileDescriptor) bool {
			count++
			return false
		})
	if count != 0 {
		atom.Log.Debugf("use already injected protoregistry.GlobalFiles")
		return prFiles, nil
	}
	return xproto.NewFiles(importPaths, protoFiles...)
}
