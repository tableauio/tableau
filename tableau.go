package tableau

import (
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/protogen"
	"github.com/tableauio/tableau/internal/xlsxgen"
	"github.com/tableauio/tableau/options"
)

// Generate can convert Excel/CSV/XML files to protoconf files and
// different configuration files: JSON, Text, and Wire at the same time.
func Generate(protoPackage, indir, outdir string, setters ...options.Option) {
	GenProto(protoPackage, indir, outdir, setters...)
	GenConf(protoPackage, indir, outdir, setters...)
}

// GenProto can convert Excel/CSV/XML files to protoconf files.
func GenProto(protoPackage, indir, outdir string, setters ...options.Option) {
	opts := options.ParseOptions(setters...)
	g := protogen.NewGenerator(protoPackage, indir, outdir, setters...)
	atom.InitZap(opts.LogLevel)
	atom.Log.Debugf("options inited: %+v, header: %+v, output: %+v", opts, opts.Header, opts.Output)
	if err := g.Generate(); err != nil {
		red := color.New(color.FgRed).SprintfFunc()
		atom.Log.Errorf(red("generate failed: %+v", err))
		os.Exit(-1)
	}
}

// GenConf can convert Excel/CSV/XML files to different configuration files: JSON, Text, and Wire.
func GenConf(protoPackage, indir, outdir string, setters ...options.Option) {
	opts := options.ParseOptions(setters...)
	g := confgen.NewGenerator(protoPackage, indir, outdir, setters...)
	atom.InitZap(opts.LogLevel)
	atom.Log.Debugf("options inited: %+v, header: %+v, output: %+v", opts, opts.Header, opts.Output)
	if err := g.Generate(opts.Workbook, opts.Worksheet); err != nil {
		red := color.New(color.FgRed).SprintfFunc()
		atom.Log.Errorf(red("generate failed: %+v", err))
		os.Exit(-1)
	}
}

// Proto2Excel converts protoconf files to excel files (with tableau header).
func Proto2Excel(protoPackage, indir, outdir string) {
	g := xlsxgen.Generator{
		ProtoPackage: protoPackage,
		InputDir:     indir,
		OutputDir:    outdir,
	}
	g.Generate()
}

// ParseMeta parses the metasheet "@TABLEAU" in a workbook.
func ParseMeta(indir, relWorkbookPath string) (importer.Importer, error) {
	parser := confgen.NewSheetParser(protogen.TableauProtoPackage, "", book.MetasheetOptions())
	return importer.New(
		filepath.Join(indir, relWorkbookPath),
		importer.Parser(parser),
	)
}
