package store

type Options struct {
	// Specify output file name (without file extension).
	//
	// Default: "".
	Name string
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
}

// Option is the functional option type.
type Option func(*Options)

// Name specifies the output file name (without file extension).
func Name(v string) Option {
	return func(opts *Options) {
		opts.Name = v
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
