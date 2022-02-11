package tableau

import (
	"os"
	"path/filepath"

	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/protogen"
	"github.com/tableauio/tableau/internal/xlsxgen"
	"github.com/tableauio/tableau/options"
)

// Excel2Conf converts excel files (with tableau header) to different formatted configuration files.
// Supported formats: json, prototext, and protowire.
func Excel2Conf(protoPackage, indir, outdir string, setters ...options.Option) {
	opts := options.ParseOptions(setters...)
	g := confgen.NewGenerator(protoPackage, indir, outdir, setters...)
	atom.InitZap(opts.LogLevel)
	atom.Log.Debugf("options inited: %+v, header: %+v, output: %+v", opts, opts.Header, opts.Output)
	if err := g.Generate(opts.Workbook, opts.Worksheet); err != nil {
		atom.Log.Errorf("generate failed: %+v", err)
		os.Exit(-1)
	}
}

// Excel2Proto converts excel files (with tableau header) to protoconf files.
func Excel2Proto(protoPackage, goPackage, indir, outdir string, setters ...options.Option) {
	opts := options.ParseOptions(setters...)
	g := protogen.NewGenerator(protoPackage, goPackage, indir, outdir, setters...)
	atom.InitZap(opts.LogLevel)
	atom.Log.Debugf("options inited: %+v, header: %+v, output: %+v", opts, opts.Header, opts.Output)
	if err := g.Generate(); err != nil {
		atom.Log.Errorf("generate failed: %+v", err)
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

// ParseMeta parses the @TABLEAU sheet in a workboot.
func ParseMeta(indir, relWorkbookPath string) importer.Importer {
	parser := confgen.NewSheetParser(protogen.TableauProtoPackage, "")
	return importer.New(
		filepath.Join(indir, relWorkbookPath),
		importer.Parser(parser),
	)
}

// XML2Conf converts xml files to different formatted configuration files.
// Supported formats: json, prototext, and protowire.
func XML2Conf(protoPackage, indir, outdir string, setters ...options.Option) {
	opts := options.ParseOptions(setters...)
	g := confgen.NewGenerator(protoPackage, indir, outdir, setters...)
	atom.InitZap(opts.LogLevel)
	atom.Log.Debugf("options inited: %+v, header: %+v, output: %+v", opts, opts.Header, opts.Output)
	if err := g.Generate(opts.Workbook, opts.Worksheet); err != nil {
		atom.Log.Errorf("generate failed: %+v", err)
		os.Exit(-1)
	}
}

// XML2Proto converts xml files to protoconf files.
func XML2Proto(protoPackage, goPackage, indir, outdir string, setters ...options.Option) {
	opts := options.ParseOptions(setters...)
	g := protogen.NewGenerator(protoPackage, goPackage, indir, outdir, setters...)
	atom.InitZap(opts.LogLevel)
	atom.Log.Debugf("options inited: %+v, header: %+v, output: %+v", opts, opts.Header, opts.Output)
	if err := g.Generate(); err != nil {
		atom.Log.Errorf("generate failed: %+v", err)
		os.Exit(-1)
	}
}
