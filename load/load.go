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
	"google.golang.org/protobuf/reflect/protoregistry"
)

type Options struct {
	// Location represents the collection of time offsets in use in
	// a geographical area.
	//
	// If the name is "" or "UTC", LoadLocation returns UTC.
	// If the name is "Local", LoadLocation returns Local.
	// Default: "Local".
	LocationName string
	// IgnoreUnknownFields signifies whether to ignore unknown JSON fields
	// during parsing.
	// Default: false.
	IgnoreUnknownFields bool
	// SubdirRewrites rewrites subdir paths (relative to workbook name option
	// in .proto file).
	// Default: nil.
	SubdirRewrites map[string]string
	// Paths maps each messager name to a corresponding config file path.
	// If a messager name is existed, then the messager will be parsed from
	// the config file directly.
	// NOTE: only JSON, bin, and text formats are supported.
	// Default: nil.
	Paths map[string]string
}

// Option is the functional option type.
type Option func(*Options)

// LocationName sets TZ location name for parsing datetime format.
func LocationName(name string) Option {
	return func(opts *Options) {
		opts.LocationName = name
	}
}

// IgnoreUnknownFields ignores unknown JSON fields during parsing.
func IgnoreUnknownFields() Option {
	return func(opts *Options) {
		opts.IgnoreUnknownFields = true
	}
}

// SubdirRewrites rewrites subdir paths (relative to workbook name option
// in .proto file).
func SubdirRewrites(subdirRewrites map[string]string) Option {
	return func(opts *Options) {
		opts.SubdirRewrites = subdirRewrites
	}
}

// Paths maps each messager name to a corresponding config file path.
// If a messager name is existed, then the messager will be parsed from
// the config file directly.
func Paths(paths map[string]string) Option {
	return func(opts *Options) {
		opts.Paths = paths
	}
}

// newDefault returns a default Options.
func newDefault() *Options {
	return &Options{
		LocationName: "Local",
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

// Load reads file content from the specified directory and format,
// and then fills the provided message.
func Load(msg proto.Message, dir string, fmt format.Format, options ...Option) error {
	if format.IsInputFormat(fmt) {
		return loadOrigin(msg, dir, options...)
	}

	var path string
	name := string(msg.ProtoReflect().Descriptor().Name())
	opts := ParseOptions(options...)
	if opts.Paths != nil {
		if p, ok := opts.Paths[name]; ok {
			// path specified explicitly, then use it directly
			path = p
			fmt = format.Ext2Format(filepath.Ext(path))
		}
	}
	if path == "" {
		switch fmt {
		case format.JSON:
			path = filepath.Join(dir, name+format.JSONExt)
		case format.Text:
			path = filepath.Join(dir, name+format.TextExt)
		case format.Bin:
			path = filepath.Join(dir, name+format.BinExt)
		default:
			return errors.Errorf("unknown format: %v", fmt)
		}
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return errors.Wrapf(err, "failed to read file: %v", path)
	}
	switch fmt {
	case format.JSON:
		unmarshOpts := protojson.UnmarshalOptions{
			DiscardUnknown: opts.IgnoreUnknownFields,
		}
		return unmarshOpts.Unmarshal(content, msg)
	case format.Text:
		return prototext.Unmarshal(content, msg)
	case format.Bin:
		return proto.Unmarshal(content, msg)
	default:
		return errors.Errorf("unknown format: %v", fmt)
	}
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
	_, wsOpts := confgen.ParseMessageOptions(md)
	sheets := []string{wsOpts.Name}

	self, err := importer.New(
		wbPath,
		importer.Sheets(sheets),
	)
	if err != nil {
		return errors.WithMessagef(err, "failed to import workbook: %v", wbPath)
	}

	// get merger importer infos
	impInfos, err := importer.GetMergerImporters(dir, workbook.Name, wsOpts.Name, wsOpts.Merger, opts.SubdirRewrites)
	if err != nil {
		return errors.WithMessagef(err, "failed to get merger importer infos for %s", wbPath)
	}
	// append self
	impInfos = append(impInfos, importer.ImporterInfo{Importer: self})

	sheetInfo := &confgen.SheetInfo{
		ProtoPackage:    string(md.ParentFile().Package()),
		LocationName:    opts.LocationName,
		PrimaryBookName: workbook.Name,
		MD:              md,
		Opts:            wsOpts,
		ExtInfo: &confgen.SheetParserExtInfo{
			InputDir:       dir,
			SubdirRewrites: opts.SubdirRewrites,
			PRFiles:        protoregistry.GlobalFiles,
		},
	}
	protomsg, err := confgen.ParseMessage(sheetInfo, impInfos...)
	if err != nil {
		return err
	}
	// NOTE: deep copy
	proto.Merge(msg, protomsg)
	return nil
}
