package confgen

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/xproto"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
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

	OutputOpt *options.OutputOption // output settings.
	InputOpt  *options.InputOption  // Input settings.
	Header    *options.HeaderOption // header settings.
}

var specialMessageMap = map[string]int{
	"google.protobuf.Timestamp": 1,
	"google.protobuf.Duration":  1,
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
		InputOpt:     opts.Input,
		OutputOpt:    opts.Output,
	}
	return g
}

type sheetInfo struct {
	MessageName string
	opts        *tableaupb.WorksheetOptions
}

func (gen *Generator) Generate(relWorkbookPath string, worksheetName string) (err error) {
	if relWorkbookPath != "" {
		relCleanSlashPath := fs.GetCleanSlashPath(relWorkbookPath)
		if err != nil {
			return errors.Wrapf(err, "failed to get relative path from %s to %s", gen.InputDir, relWorkbookPath)
		}
		atom.Log.Debugf("convert relWorkbookPath to relCleanSlashPath: %s -> %s", relWorkbookPath, relCleanSlashPath)
		relWorkbookPath = relCleanSlashPath
	}

	// create output dir
	outputConfDir := filepath.Join(gen.OutputDir, gen.OutputOpt.ConfSubdir)
	err = os.MkdirAll(outputConfDir, 0700)
	if err != nil {
		return errors.WithMessagef(err, "failed to create output dir: %s", outputConfDir)
	}

	workbookFound := false
	worksheetFound := false

	prFiles, err := getProtoRegistryFiles(gen.ProtoPackage, gen.InputOpt.ImportPaths, gen.InputOpt.ProtoFiles...)
	if err != nil {
		return errors.WithMessagef(err, "failed to create files")
	}

	prFiles.RangeFilesByPackage(
		protoreflect.FullName(gen.ProtoPackage),
		func(fd protoreflect.FileDescriptor) bool {
			err = func() error {
				_, workbook := ParseFileOptions(fd)
				if workbook == nil {
					return nil
				}
				if relWorkbookPath != "" && relWorkbookPath != workbook.Name {
					return nil
				}
				workbookFound = true

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
				imp, err := importer.New(wbPath, importer.Sheets(sheets))
				if err != nil {
					return errors.WithMessagef(err, "failed to import workbook: %s", wbPath)
				}
				// atom.Log.Debugf("proto: %s, workbook %s", fd.Path(), workbook)
				for sheetName, sheetInfo := range sheetMap {
					if worksheetName != "" && worksheetName != sheetName {
						continue
					}
					worksheetFound = true

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
				}
				return nil
			}()

			// Due to closure, this err will be returned by func Generate().
			return err == nil
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
	if !worksheetFound {
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
