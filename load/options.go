package load

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
