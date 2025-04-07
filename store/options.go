package store

type Options struct {
	// Filter can only filter in certain specific messagers based on the
	// condition that you provide.
	//
	// NOTE: only used in https://github.com/tableauio/loader.
	//
	// Default: nil.
	Filter FilterFunc

	// Specify output file name (without file extension).
	//
	// Default: "".
	Name string
	// Location represents the collection of time offsets in use in a geographical area.
	//  - If the name is "" or "UTC", LoadLocation returns UTC.
	//  - If the name is "Local", LoadLocation returns Local.
	//  - Otherwise, the name is taken to be a location name corresponding to a file in the
	//    IANA Time Zone database, such as "America/New_York", "Asia/Shanghai", and so on.
	//
	// See https://go.dev/src/time/zoneinfo_abbrs_windows.go.
	//
	// Default: "Local".
	LocationName string `yaml:"locationName"`
	// Output pretty format of JSON and Text, with multiline and indent.
	//
	// Default: false.
	Pretty bool
	// EmitUnpopulated specifies whether to emit unpopulated fields. It does not
	// emit unpopulated oneof fields or unpopulated extension fields.
	// The JSON value emitted for unpopulated fields are as follows:
	//  ╔═══════╤════════════════════════════╗
	//  ║ JSON  │ Protobuf field             ║
	//  ╠═══════╪════════════════════════════╣
	//  ║ false │ proto3 boolean fields      ║
	//  ║ 0     │ proto3 numeric fields      ║
	//  ║ ""    │ proto3 string/bytes fields ║
	//  ║ null  │ proto2 scalar fields       ║
	//  ║ null  │ message fields             ║
	//  ║ []    │ list fields                ║
	//  ║ {}    │ map fields                 ║
	//  ╚═══════╧════════════════════════════╝
	//
	// NOTE: worksheet with FieldPresence set as true ignore this option.
	//
	// Refer: https://github.com/protocolbuffers/protobuf/blob/main/docs/field_presence.md
	//
	// Default: false.
	EmitUnpopulated bool
	// UseProtoNames uses proto field name instead of lowerCamelCase name in JSON
	// field names.
	UseProtoNames bool
	// UseEnumNumbers emits enum values as numbers.
	UseEnumNumbers bool
	// UseTimezones specifies whether to emit timestamp in string format with
	// timezones (as indicated by an offset).
	//
	// NOTE: use with option "LocationName".
	UseTimezones bool
}

// FilterFunc filter in messagers if returned value is true.
//
// NOTE: name is the protobuf message name, e.g.: "message ItemConf{...}".
//
// FilterFunc is redefined here (also defined in "load" package) to avoid
// "import cycle" problem.
type FilterFunc func(name string) bool

// Option is the functional option type.
type Option func(*Options)

// newDefault returns a default Options.
func newDefault() *Options {
	return &Options{}
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

// Filter can only filter in certain specific messagers based on the
// condition that you provide.
//
// NOTE: only used in https://github.com/tableauio/loader.
func Filter(filter FilterFunc) Option {
	return func(opts *Options) {
		opts.Filter = filter
	}
}

// Name specifies the output file name (without file extension).
func Name(v string) Option {
	return func(opts *Options) {
		opts.Name = v
	}
}

// LocationName specifies the location name for timezone processing.
func LocationName(v string) Option {
	return func(opts *Options) {
		opts.LocationName = v
	}
}

// Pretty specifies whether to prettify JSON and Text output with
// multiline and indent.
func Pretty(v bool) Option {
	return func(opts *Options) {
		opts.Pretty = v
	}
}

// EmitUnpopulated specifies whether to emit unpopulated fields.
func EmitUnpopulated(v bool) Option {
	return func(opts *Options) {
		opts.EmitUnpopulated = v
	}
}

// UseProtoNames specifies whether to use proto field name instead of
// lowerCamelCase name in
// JSON field names.
func UseProtoNames(v bool) Option {
	return func(opts *Options) {
		opts.UseProtoNames = v
	}
}

// UseEnumNumbers specifies whether to emit enum values as numbers for
// JSON field values.
func UseEnumNumbers(v bool) Option {
	return func(opts *Options) {
		opts.UseEnumNumbers = v
	}
}

// UseTimezones specifies whether to emit timestamp in string format with
// timezones (as indicated by an offset).
func UseTimezones(v bool) Option {
	return func(opts *Options) {
		opts.UseTimezones = v
	}
}
