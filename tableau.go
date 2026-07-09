package tableau

import (
	"context"

	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/localizer"
	"github.com/tableauio/tableau/internal/protogen"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
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

// SetLogger installs a user-provided logger as tableau's log destination,
// so that log output produced by tableau (including when invoked
// indirectly, e.g. via load.LoadMessagerInDir) can be routed into the
// caller's own logging system.
//
// logger only needs to implement 7 Printf-style methods (Debugf, Infof,
// Warnf, Errorf, DPanicf, Panicf, Fatalf; see log.Logger), so most loggers
// (e.g. *zap.SugaredLogger) satisfy it directly, and others (e.g. slog,
// logrus) can be adapted with a thin wrapper.
//
// Once set, it takes effect immediately and is not overridden by the log
// options (options.Log) passed to Generate/GenProto/GenConf, regardless of
// call order.
func SetLogger(logger log.Logger) {
	log.SetLogger(logger)
}

// NewImporter creates a new importer of the specified workbook.
func NewImporter(workbookPath string) (importer.Importer, error) {
	ctx := context.Background()
	parser := confgen.NewSheetParser(ctx, xproto.InternalProtoPackage, "", book.MetasheetOptions(ctx))
	return importer.New(ctx, workbookPath, importer.Parser(parser))
}
