package tableau

import (
	"path/filepath"

	"github.com/davecgh/go-spew/spew"
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
func GenProto(protoPackage, indir, outdir string, setters ...options.Option) error {
	opts := options.ParseOptions(setters...)
	atom.InitConsoleLog(opts.LogLevel)
	g := protogen.NewGenerator(protoPackage, indir, outdir, setters...)
	atom.Log.Debugf("options inited: %+v", spew.Sdump(opts))
	return g.Generate()
}

// GenConf can convert Excel/CSV/XML files to different configuration files: JSON, Text, and Wire.
func GenConf(protoPackage, indir, outdir string, setters ...options.Option) error {
	opts := options.ParseOptions(setters...)
	atom.InitConsoleLog(opts.LogLevel)
	g := confgen.NewGenerator(protoPackage, indir, outdir, setters...)
	atom.Log.Debugf("options inited: %+v", spew.Sdump(opts))
	return g.Generate(opts.Workbook, opts.Worksheet)
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

// SetLog set the log level and path for debugging.
// If dir is empty, the log will be written to console,
// otherwise it will be written to files in dir.
func SetLog(logLevel, dir string) error {
	if dir == "" {
		return atom.InitConsoleLog(logLevel)
	}
	return atom.InitFileLog(logLevel, dir, "tableau.log")
}
