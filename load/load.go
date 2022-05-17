package load

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

type Options struct {
	// Rewrite subdir path (relative to workbook name option in .proto file).
	// Default: nil.
	SubdirRewrites map[string]string
}

// Option is the functional option type.
type Option func(*Options)

// SubdirRewrites option.
func SubdirRewrites(subdirRewrites map[string]string) Option {
	return func(opts *Options) {
		opts.SubdirRewrites = subdirRewrites
	}
}

// newDefault returns a default Options.
func newDefault() *Options {
	return &Options{
		SubdirRewrites: nil,
	}
}

// ParseOptions parses functional options and merge them to default Options.
func ParseOptions(setters ...Option) *Options {
	// Default Options
	opts := newDefault()
	for _, setter := range setters {
		setter(opts)
	}
	return opts
}

func Load(msg proto.Message, dir string, fmt format.Format, options ...Option) error {
	switch fmt {
	case format.JSON:
		return loadJSON(msg, dir, options...)
	case format.Text:
		return loadText(msg, dir, options...)
	case format.Wire:
		return loadWire(msg, dir, options...)
	case format.Excel, format.CSV, format.XML:
		return loadOrigin(msg, dir, options...)
	default:
		return errors.Errorf("unknown format: %v", fmt)
	}
}

func loadJSON(msg proto.Message, dir string, options ...Option) error {
	msgName := string(msg.ProtoReflect().Descriptor().Name())
	path := filepath.Join(dir, msgName+format.JSONExt)

	if content, err := os.ReadFile(path); err != nil {
		return errors.Wrapf(err, "failed to read file: %v", path)
	} else {
		if err := protojson.Unmarshal(content, msg); err != nil {
			return errors.Wrapf(err, "failed to unmarhsal message: %v", msgName)
		}
	}
	return nil
}

func loadText(msg proto.Message, dir string, options ...Option) error {
	msgName := string(msg.ProtoReflect().Descriptor().Name())
	path := filepath.Join(dir, msgName+format.TextExt)

	if content, err := os.ReadFile(path); err != nil {
		return errors.Wrapf(err, "failed to read file: %v", path)
	} else {
		if err := prototext.Unmarshal(content, msg); err != nil {
			return errors.Wrapf(err, "failed to unmarhsal message: %v", msgName)
		}
	}
	return nil
}

func loadWire(msg proto.Message, dir string, options ...Option) error {
	msgName := string(msg.ProtoReflect().Descriptor().Name())
	path := filepath.Join(dir, msgName+format.WireExt)

	if content, err := os.ReadFile(path); err != nil {
		return errors.Wrapf(err, "failed to read file: %v", path)
	} else {
		if err := proto.Unmarshal(content, msg); err != nil {
			return errors.Wrapf(err, "failed to unmarhsal message: %v", msgName)
		}
	}
	return nil
}

// loadOrigin loads the origin file(excel/csv/xml) from the given directory.
func loadOrigin(msg proto.Message, dir string, options ...Option) error {
	opts := ParseOptions(options...)

	md := msg.ProtoReflect().Descriptor()
	protofile, workbook := confgen.ParseFileOptions(md.ParentFile())
	if workbook == nil {
		return errors.Errorf("workbook options not found of protofile: %v", protofile)
	}
	// rewrite subdir
	rewrittenWorkbookName := fs.RewriteSubdir(workbook.Name, opts.SubdirRewrites)
	wbPath := filepath.Join(dir, rewrittenWorkbookName)
	atom.Log.Debugf("load origin file: %v", wbPath)
	// get sheet name
	msgName, wsOpts := confgen.ParseMessageOptions(md)
	sheets := []string{wsOpts.Name}

	imp, err := importer.New(
		wbPath,
		importer.Sheets(sheets),
	)
	if err != nil {
		return errors.WithMessagef(err, "failed to import workbook: %v", wbPath)
	}

	sheet := imp.GetSheet(wsOpts.Name)
	if sheet == nil {
		return errors.WithMessagef(err, "%v|sheet %s not found", msgName, wsOpts.Name)
	}
	pkgName := md.ParentFile().Package()
	// TODO: support LocationName setting by using Functional Options
	locationName := ""
	parser := confgen.NewSheetParser(string(pkgName), locationName, wsOpts)
	if err := parser.Parse(msg, sheet); err != nil {
		return errors.WithMessagef(err, "%v|failed to parse sheet: %s", msgName, wsOpts.Name)
	}
	return nil
}
