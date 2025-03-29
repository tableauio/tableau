package tableau

import (
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/localizer"
	"github.com/tableauio/tableau/internal/protogen"
	"github.com/tableauio/tableau/internal/strcase"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/xerrors"
)

// Generate converts Excel/CSV/XML/YAML files to protoconf files and
// different configuration files: JSON, Text, and Bin.
func Generate(protoPackage, indir, outdir string, setters ...options.Option) error {
	if err := GenProto(protoPackage, indir, outdir, setters...); err != nil {
		return xerrors.Wrapf(err, "failed to generate proto files")
	}
	if err := GenConf(protoPackage, indir, outdir, setters...); err != nil {
		return xerrors.Wrapf(err, "failed to generate conf files")
	}
	return nil
}

// GenProto converts Excel/CSV/XML/YAML files to protoconf files.
func GenProto(protoPackage, indir, outdir string, setters ...options.Option) (err error) {
	opts := options.ParseOptions(setters...)
	if err := localizer.SetLang(opts.Lang); err != nil {
		return err
	}
	if err := log.Init(opts.Log); err != nil {
		return err
	}
	g := protogen.NewGenerator(protoPackage, indir, outdir, setters...)
	return g.Generate()
}

// GenConf converts Excel/CSV/XML/YAML files to different configuration files: JSON, Text, and Bin.
func GenConf(protoPackage, indir, outdir string, setters ...options.Option) error {
	opts := options.ParseOptions(setters...)
	if err := localizer.SetLang(opts.Lang); err != nil {
		return err
	}
	if err := log.Init(opts.Log); err != nil {
		return err
	}
	g := confgen.NewGenerator(protoPackage, indir, outdir, setters...)
	return g.Generate()
}

// NewProtoGenerator creates a new proto generator.
func NewProtoGenerator(protoPackage, indir, outdir string, options ...options.Option) *protogen.Generator {
	return protogen.NewGenerator(protoPackage, indir, outdir, options...)
}

// NewProtoGeneratorWithOptions creates a new proto generator with options.
func NewProtoGeneratorWithOptions(protoPackage, indir, outdir string, options *options.Options) *protogen.Generator {
	return protogen.NewGeneratorWithOptions(protoPackage, indir, outdir, options)
}

// NewConfGenerator creates a new conf generator.
func NewConfGenerator(protoPackage, indir, outdir string, options ...options.Option) *confgen.Generator {
	return confgen.NewGenerator(protoPackage, indir, outdir, options...)
}

// NewConfGeneratorWithOptions creates a new conf generator with options.
func NewConfGeneratorWithOptions(protoPackage, indir, outdir string, options *options.Options) *confgen.Generator {
	return confgen.NewGeneratorWithOptions(protoPackage, indir, outdir, options)
}

// SetLang sets the default language.
// E.g: en, zh.
func SetLang(lang string) error {
	return localizer.SetLang(lang)
}

// NewImporter creates a new importer of the specified workbook.
func NewImporter(workbookPath string) (importer.Importer, error) {
	parser := confgen.NewSheetParser(xproto.InternalProtoPackage, "", strcase.Context{}, book.MetasheetOptions())
	return importer.New(workbookPath, importer.Parser(parser))
}
