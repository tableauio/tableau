package load

import "os"

type Options struct {
	// ReadFunc reads the config file and returns its content.
	//
	// Default: os.ReadFile.
	ReadFunc ReadFunc
	// Location represents the collection of time offsets in use in
	// a geographical area.
	//
	// If the name is "" or "UTC", LoadLocation returns UTC.
	// If the name is "Local", LoadLocation returns Local.
	//
	// Default: "Local".
	LocationName string
	// IgnoreUnknownFields signifies whether to ignore unknown JSON fields
	// during parsing.
	//
	// Default: false.
	IgnoreUnknownFields bool
	// SubdirRewrites rewrites subdir paths (relative to workbook name option
	// in .proto file).
	//
	// Default: nil.
	SubdirRewrites map[string]string
	// Paths maps each messager name to a corresponding config file path.
	// If specified, then the main messager will be parsed from the file
	// directly, other than the specified load dir.
	//
	// NOTE: only JSON, Bin, and Text formats are supported.
	//
	// Default: nil.
	Paths map[string]string
	// PatchPaths maps each messager name to one or multiple corresponding patch file paths.
	// If specified, then main messager will be patched.
	//
	// NOTE: only JSON, Bin, and Text formats are supported.
	//
	// Default: nil.
	PatchPaths map[string][]string
	// PatchDirs specifies the directory paths for config patching.
	//
	// Default: nil.
	PatchDirs []string
	// Mode specifies the loading mode for config patching.
	//
	// Default: ModeDefault.
	Mode LoadMode
}

type LoadMode int

const (
	ModeDefault   LoadMode = iota // Load all related files
	ModeOnlyMain                  // Only load the main file
	ModeOnlyPatch                 // Only load the patch files
)

// ReadFunc reads the config file and returns its content.
type ReadFunc func(name string) ([]byte, error)

// Option is the functional option type.
type Option func(*Options)

// newDefault returns a default Options.
func newDefault() *Options {
	return &Options{
		LocationName: "Local",
		ReadFunc:     os.ReadFile,
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

// ReadFunc reads the config file and returns its content.
func WithReadFunc(readFunc ReadFunc) Option {
	return func(opts *Options) {
		if readFunc != nil {
			opts.ReadFunc = readFunc
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
		opts.Paths = paths
	}
}

// PatchPaths maps each messager name to one or multiple corresponding patch file paths.
// If specified, then main messager will be patched.
//
// NOTE: only JSON, Bin, and Text formats are supported.
func PatchPaths(paths map[string][]string) Option {
	return func(opts *Options) {
		opts.PatchPaths = paths
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
