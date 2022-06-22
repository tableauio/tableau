package tableau

import (
	"path/filepath"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/protogen"
	"github.com/tableauio/tableau/internal/xlsxgen"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
)

// Generate can convert Excel/CSV/XML files to protoconf files and
// different configuration files: JSON, Text, and Wire at the same time.
func Generate(protoPackage, indir, outdir string, setters ...options.Option) error {
	if err := GenProto(protoPackage, indir, outdir, setters...); err != nil {
		return errors.WithMessagef(err, "failed to generate proto files")
	}
	if err := GenConf(protoPackage, indir, outdir, setters...); err != nil {
		return errors.WithMessagef(err, "failed to generate conf files")
	}
	return nil
}

// GenProto can convert Excel/CSV/XML files to protoconf files.
func GenProto(protoPackage, indir, outdir string, setters ...options.Option) (err error) {
	opts := options.ParseOptions(setters...)
	if err := log.Init(opts.Log); err != nil {
		return err
	}
	g := protogen.NewGenerator(protoPackage, indir, outdir, setters...)
	atom.Log.Debugf("options inited: %+v", spew.Sdump(opts))
	return g.Generate()
}

// GenConf can convert Excel/CSV/XML files to different configuration files: JSON, Text, and Wire.
func GenConf(protoPackage, indir, outdir string, setters ...options.Option) error {
	opts := options.ParseOptions(setters...)
	if err := log.Init(opts.Log); err != nil {
		return err
	}
	g := confgen.NewGenerator(protoPackage, indir, outdir, setters...)
	atom.Log.Debugf("options inited: %+v", spew.Sdump(opts))
	return g.Generate()
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


