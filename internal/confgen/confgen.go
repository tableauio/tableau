package confgen

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
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

	Output *options.OutputOption // output settings.
	Input  *options.InputOption  // Input settings.
	Header *options.HeaderOption // header settings.
}

var specialMessageMap = map[string]int{
	"google.protobuf.Timestamp": 1,
	"google.protobuf.Duration":  1,
}

func NewGenerator(protoPackage, indir, outdir string, setters ...options.Option) *Generator {
	opts := options.ParseOptions(setters...)
	g := &Generator{
		ProtoPackage: protoPackage,
		LocationName: opts.LocationName,
		InputDir:     indir,
		OutputDir:    outdir,
		Output:       opts.Output,
		Input:        opts.Input,
		Header:       opts.Header,
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
	err = os.MkdirAll(gen.OutputDir, 0700)
	if err != nil {
		return errors.WithMessagef(err, "failed to create output dir: %s", gen.OutputDir)
	}

	workbookFound := false
	worksheetFound := false

	protoregistry.GlobalFiles.RangeFilesByPackage(
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
				if !format.NeedProcessInput(workbookFormat, gen.Input.Formats) {
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
				wbPath := filepath.Join(gen.InputDir, workbook.Name)
				imp := importer.New(wbPath, importer.Sheets(sheets), importer.Header(gen.Header))
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
					exporter := NewSheetExporter(gen.OutputDir, gen.Output)
					err := exporter.Export(imp, parser, newMsg)
					if err != nil {
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
