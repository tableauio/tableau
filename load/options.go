package load

import "google.golang.org/protobuf/reflect/protoreflect"

type Options struct {
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
	// PatchPaths maps each messager name to a corresponding patch file path.
	// If specified, then main messager will patched.
	//
	// NOTE: only JSON, Bin, and Text formats are supported.
	//
	// Default: nil.
	PatchPaths map[string]string
	// PatchDir specifies the directory path for config patching. If not
	// specified, then no patching will be applied.
	//
	// Default: "".
	PatchDir string
	// Filter can only filter in certain specific messagers based on the
	// condition that you provide.
	//
	// NOTE:
	//	- messagers specified in "Paths" cannot be patched.
	//  - only used in https://github.com/tableauio/loader.
	//
	// Default: nil.
	Filter FilterFunc
}

// FilterFunc filter in messagers if returned value is true.
type FilterFunc func(msg protoreflect.FullName) bool

// Option is the functional option type.
type Option func(*Options)

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

// PatchPaths maps each messager name to a corresponding patch file path.
// If specified, then main messager will patched.
//
// NOTE: only JSON, Bin, and Text formats are supported.
func PatchPaths(paths map[string]string) Option {
	return func(opts *Options) {
		opts.PatchPaths = paths
	}
}

// PatchDir specifies the directory path for config patching. If not
// specified, then no patching will be applied.
func PatchDir(dir string) Option {
	return func(opts *Options) {
		opts.PatchDir = dir
	}
}

// Filter can only filter in certain specific messagers based on the
// condition that you provide.
//
// NOTE:
//	- messagers specified in "Paths" cannot be patched.
//  - only used in https://github.com/tableauio/loader.
//
func Filter(filter FilterFunc) Option {
	return func(opts *Options) {
		opts.Filter = filter
	}
}
