package load

import (
	"github.com/tableauio/tableau/format"
	"google.golang.org/protobuf/proto"
)

// BaseOptions is the common struct for both global-level and sheet-level
// Options.
type BaseOptions struct {
	// LoadFunc loads a messager's content.
	//
	// NOTE: only output formats(JSON, Bin, Text) are supported.
	//
	// Default: load.
	LoadFunc LoadFn
}

type MessagerOptions struct {
	BaseOptions
	// Path specifies a this messager's config file path.
	// If specified, then the main messager will be parsed from the file
	// directly, other than the specified load dir.
	//
	// NOTE: only output formats(JSON, Bin, Text) are supported.
	//
	// Default: nil.
	Path string
	// PatchPaths specifies one or multiple corresponding patch file paths.
	// If specified, then main messager will be patched.
	//
	// NOTE: only output formats(JSON, Bin, Text) are supported.
	//
	// Default: nil.
	PatchPaths []string
}

type Options struct {
	BaseOptions
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
	// Default: false.
	IgnoreUnknownFields bool
	// SubdirRewrites rewrites subdir paths (relative to workbook name option
	// in .proto file).
	//
	// NOTE: only input formats(Excel, CSV, XML, YAML) are supported.
	//
	// Default: nil.
	SubdirRewrites map[string]string
	// PatchDirs specifies the directory paths for config patching.
	//
	// NOTE: only output formats(JSON, Bin, Text) are supported.
	//
	// Default: nil.
	PatchDirs []string
	// Mode specifies the loading mode for config patching.
	//
	// NOTE: only output formats(JSON, Bin, Text) are supported.
	//
	// Default: ModeDefault.
	Mode LoadMode
	// MessagerOptions maps each messager name to a MessageOptions.
	// If specified, then the messager will be parsed with the given options
	// directly.
	//
	// Default: nil.
	MessagerOptions map[string]*MessagerOptions
}

type LoadMode int

const (
	ModeDefault   LoadMode = iota // Load all related files
	ModeOnlyMain                  // Only load the main file
	ModeOnlyPatch                 // Only load the patch files
)

// LoadFn loads the config file to the input msg.
type LoadFn func(proto.Message, string, format.Format, *Options) error

func (o *Options) GetLoadFunc(name string) LoadFn {
	if opts := o.MessagerOptions[name]; opts != nil && opts.LoadFunc != nil {
		return opts.LoadFunc
	}
	if o.LoadFunc != nil {
		return o.LoadFunc
	}
	return defaultLoad
}

// Option is the functional option type.
type Option func(*Options)

// newDefault returns a default Options.
func newDefault() *Options {
	return &Options{
		LocationName:    "Local",
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

// LoadFunc loads a messager's content.
func LoadFunc(loadFunc LoadFn) Option {
	return func(opts *Options) {
		opts.LoadFunc = loadFunc
	}
}

// LoadFuncs maps each messager name to a corresponding LoadFn.
// If specified, then the main messager will be loaded by the current
// load func, other than the global one.
func LoadFuncs(loadFuncs map[string]LoadFn) Option {
	return func(opts *Options) {
		for name, loadFunc := range loadFuncs {
			if opts.MessagerOptions[name] == nil {
				opts.MessagerOptions[name] = &MessagerOptions{}
			}
			opts.MessagerOptions[name].LoadFunc = loadFunc
		}
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
// If specified, then the main messager will be parsed from the file
// directly, other than the specified load dir.
//
// NOTE: only JSON, Bin, and Text formats are supported.
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
