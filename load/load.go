package load

import (
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/confgen"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/log"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
)

type Options struct {
	// Rewrite subdir path (relative to workbook name option in .proto file).
	// Default: nil.
	SubdirRewrites map[string]string
	// Location represents the collection of time offsets in use in a geographical area.
	// If the name is "" or "UTC", LoadLocation returns UTC.
	// If the name is "Local", LoadLocation returns Local.
	// Default: "Local".
	LocationName string
}

// Option is the functional option type.
type Option func(*Options)

// SubdirRewrites option.
func SubdirRewrites(subdirRewrites map[string]string) Option {
	return func(opts *Options) {
		opts.SubdirRewrites = subdirRewrites
	}
}

// LocationName sets TZ location name for parsing datetime format.
func LocationName(o string) Option {
	return func(opts *Options) {
		opts.LocationName = o
	}
}

// newDefault returns a default Options.
func newDefault() *Options {
	return &Options{
		SubdirRewrites: nil,
		LocationName:   "Local",
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
	log.Debugf("load origin file: %v", wbPath)
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

	// get merger importers
	importers, err := confgen.GetMergerImporters(wbPath, wsOpts.Name, wsOpts.Merger)
	if err != nil {
		return errors.WithMessagef(err, "failed to get merger importers for %s", wbPath)
	}
	// append self
	importers = append(importers, imp)

	parser := confgen.NewSheetParser(string(md.ParentFile().Package()), opts.LocationName, wsOpts)
	if err := confgen.ParseMessage(parser, msg, wsOpts.Name, importers...); err != nil {
		return errors.WithMessagef(err, "failed to parse message %s", msgName)
	}
	return nil
}
