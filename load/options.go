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
	LocationName *string

	// IgnoreUnknownFields signifies whether to ignore unknown JSON fields
	// during parsing.
	//
	// NOTE: only JSON format is supported.
	//
	// Default: false.
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
	// Default: ModeAll.
	Mode *LoadMode

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

	// SubdirRewrites rewrites subdir paths (relative to workbook name option
	// in .proto file).
	//
	// NOTE: only input formats (Excel, CSV, XML, YAML) are supported.
	//
	// Default: nil.
	SubdirRewrites map[string]string
}

// GetLocationName returns the location name.
func (o *BaseOptions) GetLocationName() string {
	if o.LocationName == nil {
		return "Local"
	}
	return *o.LocationName
}

// GetIgnoreUnknownFields returns whether to ignore unknown fields when loading
// JSON.
func (o *BaseOptions) GetIgnoreUnknownFields() bool {
	if o.IgnoreUnknownFields == nil {
		return false
	}
	return *o.IgnoreUnknownFields
}

// GetMode returns the loading mode.
func (o *BaseOptions) GetMode() LoadMode {
	if o.Mode == nil {
		return ModeAll
	}
	return *o.Mode
}

// GetReadFunc returns the read function.
func (o *BaseOptions) GetReadFunc() ReadFunc {
	if o.ReadFunc == nil {
		return os.ReadFile
	}
	return o.ReadFunc
}

// GetLoadFunc returns the load function.
func (o *BaseOptions) GetLoadFunc() LoadFunc {
	if o.LoadFunc == nil {
		return LoadMessager
	}
	return o.LoadFunc
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

// LoadOptions is the options struct, which contains both global-level and
// messager-level options.
type LoadOptions struct {
	BaseOptions
	// MessagerOptions maps each messager name to a MessageOptions.
	// If specified, then the messager will be parsed with the given options
	// directly.
	//
	// Default: empty.
	MessagerOptions map[string]*MessagerOptions
}

type LoadMode int

const (
	ModeAll       LoadMode = iota // Load all related files
	ModeOnlyMain                  // Only load the main file
	ModeOnlyPatch                 // Only load the patch files
)

// ReadFunc reads the config file and returns its content.
type ReadFunc func(name string) ([]byte, error)

// LoadFunc defines a func which can load message's content based on the given
// path, format, and options.
//
// NOTE: only output formats (JSON, Bin, Text) are supported.
type LoadFunc func(msg proto.Message, path string, fmt format.Format, opts *MessagerOptions) error

// ParseMessagerOptionsByName parses messager options with both global-level and
// messager-level options taken into consideration.
func ParseMessagerOptionsByName(o *LoadOptions, name string) *MessagerOptions {
	var mopts MessagerOptions
	if o == nil {
		return &mopts
	}
	if opts := o.MessagerOptions[name]; opts != nil {
		mopts = *opts
	}
	if mopts.LocationName == nil {
		mopts.LocationName = o.LocationName
	}
	if mopts.IgnoreUnknownFields == nil {
		mopts.IgnoreUnknownFields = o.IgnoreUnknownFields
	}
	if mopts.PatchDirs == nil {
		mopts.PatchDirs = o.PatchDirs
	}
	if mopts.Mode == nil {
		mopts.Mode = o.Mode
	}
	if mopts.ReadFunc == nil {
		mopts.ReadFunc = o.ReadFunc
	}
	if mopts.LoadFunc == nil {
		mopts.LoadFunc = o.LoadFunc
	}
	if mopts.SubdirRewrites == nil {
		mopts.SubdirRewrites = o.SubdirRewrites
	}
	return &mopts
}

// LoadOptions is the functional option type.
type LoadOption func(*LoadOptions)

// ParseOptions parses functional options and merge them to default Options.
func ParseOptions(setters ...LoadOption) *LoadOptions {
	// Default Options
	opts := &LoadOptions{}
	for _, setter := range setters {
		setter(opts)
	}
	return opts
}

// WithReadFunc sets a custom read func.
func WithReadFunc(readFunc ReadFunc) LoadOption {
	return func(opts *LoadOptions) {
		opts.ReadFunc = readFunc
	}
}

// WithLoadFunc sets a custom load func.
func WithLoadFunc(loadFunc LoadFunc) LoadOption {
	return func(opts *LoadOptions) {
		opts.LoadFunc = loadFunc
	}
}

// LocationName sets TZ location name for parsing datetime format.
func LocationName(name string) LoadOption {
	return func(opts *LoadOptions) {
		opts.LocationName = proto.String(name)
	}
}

// IgnoreUnknownFields ignores unknown JSON fields during parsing.
func IgnoreUnknownFields() LoadOption {
	return func(opts *LoadOptions) {
		opts.IgnoreUnknownFields = proto.Bool(true)
	}
}

// SubdirRewrites rewrites subdir paths (relative to workbook name option
// in .proto file).
func SubdirRewrites(subdirRewrites map[string]string) LoadOption {
	return func(opts *LoadOptions) {
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
func Paths(paths map[string]string) LoadOption {
	return func(opts *LoadOptions) {
		if opts.MessagerOptions == nil {
			opts.MessagerOptions = make(map[string]*MessagerOptions)
		}
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
func PatchPaths(paths map[string][]string) LoadOption {
	return func(opts *LoadOptions) {
		if opts.MessagerOptions == nil {
			opts.MessagerOptions = make(map[string]*MessagerOptions)
		}
		for name, path := range paths {
			if opts.MessagerOptions[name] == nil {
				opts.MessagerOptions[name] = &MessagerOptions{}
			}
			opts.MessagerOptions[name].PatchPaths = path
		}
	}
}

// PatchDirs specifies the directory paths for config patching.
func PatchDirs(dirs ...string) LoadOption {
	return func(opts *LoadOptions) {
		opts.PatchDirs = dirs
	}
}

// Mode specifies the loading mode for config patching.
//
// NOTE: only JSON, Bin, and Text formats are supported.
func Mode(mode LoadMode) LoadOption {
	return func(opts *LoadOptions) {
		opts.Mode = &mode
	}
}

// WithMessagerOptions sets the messager options.
func WithMessagerOptions(options map[string]*MessagerOptions) LoadOption {
	return func(opts *LoadOptions) {
		opts.MessagerOptions = options
	}
}
