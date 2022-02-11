package confgen

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/atom"
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
	Input *options.InputOption // Input settings.
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
		Input: opts.Input,
		Header: opts.Header,
	}
	return g
}

func (gen *Generator) Generate(relWorkbookPath string, worksheetName string) (err error) {
	if relWorkbookPath != "" {
		relCleanSlashPath := filepath.ToSlash(filepath.Clean(relWorkbookPath))
		if err != nil {
			return errors.Wrapf(err, "failed to get relative path from %s to %s", gen.InputDir, relWorkbookPath)
		}
		atom.Log.Debugf("relWorkbookPath: %s -> %s", relWorkbookPath, relCleanSlashPath)
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
			// atom.Log.Debugf("filepath: %s", fd.Path())
			err = func() error {
				opts := fd.Options().(*descriptorpb.FileOptions)
				workbook := proto.GetExtension(opts, tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
				if workbook == nil {
					return nil
				}
				if relWorkbookPath != "" && relWorkbookPath != workbook.Name {
					return nil
				}
				workbookFound = true

				var sheets []string
				sheetMap := map[string]string{} // sheet name -> message name
				msgs := fd.Messages()
				for i := 0; i < msgs.Len(); i++ {
					md := msgs.Get(i)
					opts := md.Options().(*descriptorpb.MessageOptions)
					worksheet := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
					if worksheet != nil {
						sheetMap[worksheet.Name] = string(md.Name())
						sheets = append(sheets, worksheet.Name)
					}
				}
				wbPath := filepath.Join(gen.InputDir, workbook.Name)
				imp := importer.New(wbPath, importer.Sheets(sheets), importer.Format(gen.Input.Format), importer.Header(gen.Header))
				// atom.Log.Debugf("proto: %s, workbook %s", fd.Path(), workbook)
				for sheetName, msgName := range sheetMap {
					if worksheetName != "" && worksheetName != sheetName {
						continue
					}
					worksheetFound = true

					md := msgs.ByName(protoreflect.Name(msgName))
					// atom.Log.Debugf("%s", md.FullName())
					atom.Log.Infof("generate: %s#%s <-> %s#%s", fd.Path(), md.Name(), workbook.Name, sheetName)
					newMsg := dynamicpb.NewMessage(md)
					parser := NewSheetParser(gen.ProtoPackage, gen.LocationName)
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
		return errors.Errorf("workbook not found: %s", relWorkbookPath)
	}
	if !worksheetFound {
		return errors.Errorf("worksheet not found: %s", worksheetName)
	}
	return nil
}
