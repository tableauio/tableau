package load

import (
	"os"

	"github.com/tableauio/tableau/format"
	"google.golang.org/protobuf/proto"
)

// BaseOptions is the common struct for both global-level and messager-level
// options.
type BaseOptions struct {
	// Location represents the collection of time offsets in use in
	// a geographical area.
	//
	// NOTE: only input formats(Excel, CSV, XML, YAML) are supported.
	//
	// If the name is "" or "UTC", LoadLocation returns UTC.
	// If the name is "Local", LoadLocation returns Local.
	//
	// Default: "Local".
	LocationName string

	// IgnoreUnknownFields signifies whether to ignore unknown JSON fields
	// during parsing.
	//
	// NOTE: only JSON format is supported.
	//
	// Default: nil.
	IgnoreUnknownFields *bool

	// PatchDirs specifies the directory paths for config patching.
	//
	// NOTE: only output formats (JSON, Bin, Text) are supported.
	//
	// Default: nil.
	PatchDirs []string

	// Mode specifies the loading mode for config patching.
	//
	// NOTE: only output formats (JSON, Bin, Text) are supported.
	//
	// Default: ModeDefault.
	Mode LoadMode

	// ReadFunc reads the config file and returns its content.
	//
	// Default: [os.ReadFile].
	ReadFunc ReadFunc

	// LoadFunc loads a messager's content.
	//
	// NOTE: only output formats (JSON, Bin, Text) are supported.
	//
	// Default: [LoadMessager].
	LoadFunc LoadFunc
}

// GetIgnoreUnknownFields returns the IgnoreUnknownFields value.
func (o *BaseOptions) GetIgnoreUnknownFields() bool {
	if o.IgnoreUnknownFields == nil {
		return false
	}
	return *o.IgnoreUnknownFields
}

// MessagerOptions is the options struct for a messager.
type MessagerOptions struct {
	BaseOptions
	// Path specifies messager's config file path.
	// If specified, then the main messager will be parsed directly,
	// other than the specified load dir.
	//
	// NOTE: only output formats(JSON, Bin, Text) are supported.
	//
	// Default: nil.
	Path string

	// PatchPaths specifies one or multiple corresponding patch file paths.
	// If specified, then main messager will be patched.
	//
	// NOTE: only output formats (JSON, Bin, Text) are supported.
	//
	// Default: nil.
	PatchPaths []string
}

// Options is the options struct, which contains both global-level and
// messager-level options.
type Options struct {
	BaseOptions
	// SubdirRewrites rewrites subdir paths (relative to workbook name option
	// in .proto file).
	//
	// NOTE: only input formats (Excel, CSV, XML, YAML) are supported.
	//
	// Default: nil.
	SubdirRewrites map[string]string
	// MessagerOptions maps each messager name to a MessageOptions.
	// If specified, then the messager will be parsed with the given options
	// directly.
	//
	// Default: nil.
	MessagerOptions map[string]*MessagerOptions
}

type LoadMode int

const (
	modeNone      LoadMode = iota // none
	ModeAll                       // Load all related files
	ModeOnlyMain                  // Only load the main file
	ModeOnlyPatch                 // Only load the patch files
)

// ReadFunc reads the config file and returns its content.
type ReadFunc func(name string) ([]byte, error)

// LoadFunc defines a func which loads message's content based on the given
// path, format, and options.
type LoadFunc func(msg proto.Message, path string, fmt format.Format, opts *MessagerOptions) error

// parseMessagerOptions parses messager options with both global-level and
// messager-level options taken into consideration.
func parseMessagerOptions(o *Options, name string) *MessagerOptions {
	var mopts *MessagerOptions
	if opts := o.MessagerOptions[name]; opts != nil {
		mopts = opts
	} else {
		mopts = &MessagerOptions{}
	}
	if mopts.BaseOptions.LocationName == "" {
		mopts.BaseOptions.LocationName = o.BaseOptions.LocationName
	}
	if mopts.BaseOptions.IgnoreUnknownFields == nil {
		mopts.BaseOptions.IgnoreUnknownFields = o.BaseOptions.IgnoreUnknownFields
	}
	if mopts.BaseOptions.PatchDirs == nil {
		mopts.BaseOptions.PatchDirs = o.BaseOptions.PatchDirs
	}
	if mopts.BaseOptions.Mode == modeNone {
		mopts.BaseOptions.Mode = o.BaseOptions.Mode
	}
	if mopts.BaseOptions.ReadFunc == nil {
		mopts.BaseOptions.ReadFunc = o.BaseOptions.ReadFunc
	}
	if mopts.BaseOptions.LoadFunc == nil {
		mopts.BaseOptions.LoadFunc = o.BaseOptions.LoadFunc
	}
	return mopts
}

// Option is the functional option type.
type Option func(*Options)

// newDefault returns a default Options.
func newDefault() *Options {
	return &Options{
		BaseOptions: BaseOptions{
			LocationName: "Local",
			Mode:         ModeAll,
			ReadFunc:     os.ReadFile,
			LoadFunc:     LoadMessager,
		},
		MessagerOptions: map[string]*MessagerOptions{},
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

// WithReadFunc sets a custom read func.
func WithReadFunc(readFunc ReadFunc) Option {
	return func(opts *Options) {
		opts.ReadFunc = readFunc
	}
}

// WithLoadFunc sets a custom load func.
func WithLoadFunc(loadFunc LoadFunc) Option {
	return func(opts *Options) {
		opts.LoadFunc = loadFunc
	}
}

// LocationName sets TZ location name for parsing datetime format.
func LocationName(name string) Option {
	return func(opts *Options) {
		opts.LocationName = name
	}
}

// IgnoreUnknownFields ignores unknown JSON fields during parsing.
func IgnoreUnknownFields() Option {
	return func(opts *Options) {
		opts.IgnoreUnknownFields = proto.Bool(true)
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
// If specified, then the main messager will be parsed from the file
// directly, other than the specified load dir.
//
// NOTE: only JSON, Bin, and Text formats are supported.
//
// Deprecated: use [WithMessagerOptions] instead.
func Paths(paths map[string]string) Option {
	return func(opts *Options) {
		for name, path := range paths {
			if opts.MessagerOptions[name] == nil {
				opts.MessagerOptions[name] = &MessagerOptions{}
			}
			opts.MessagerOptions[name].Path = path
		}
	}
}

// PatchPaths maps each messager name to one or multiple corresponding patch
// file paths. If specified, then main messager will be patched.
//
// NOTE: only JSON, Bin, and Text formats are supported.
//
// Deprecated: use [WithMessagerOptions] instead.
func PatchPaths(paths map[string][]string) Option {
	return func(opts *Options) {
		for name, path := range paths {
			if opts.MessagerOptions[name] == nil {
				opts.MessagerOptions[name] = &MessagerOptions{}
			}
			opts.MessagerOptions[name].PatchPaths = path
		}
	}
}

// PatchDirs specifies the directory paths for config patching.
func PatchDirs(dirs ...string) Option {
	return func(opts *Options) {
		opts.PatchDirs = dirs
	}
}

// Mode specifies the loading mode for config patching.
//
// NOTE: only JSON, Bin, and Text formats are supported.
func Mode(mode LoadMode) Option {
	return func(opts *Options) {
		opts.Mode = mode
	}
}

// WithMessagerOptions sets the messager options.
func WithMessagerOptions(mopts map[string]*MessagerOptions) Option {
	return func(opts *Options) {
		opts.MessagerOptions = mopts
	}
}
