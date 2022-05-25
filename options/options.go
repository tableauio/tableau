package options

import (
	"github.com/tableauio/tableau/format"
)

// Options is the wrapper of tableau params.
// Options follow the design of Functional Options (https://github.com/tmrts/go-patterns/blob/master/idiom/functional-options.md).
type Options struct {
	// Location represents the collection of time offsets in use in a geographical area.
	// If the name is "" or "UTC", LoadLocation returns UTC.
	// If the name is "Local", LoadLocation returns Local.
	// Default: "Local".
	LocationName string `yaml:"locationName"`

	Log *LogOption // Log options.

	Input  *InputOption  `yaml:"input"`  // Input options.
	Output *OutputOption `yaml:"output"` // Output options.
}

type LogOption struct {
	// Log level: DEBUG, INFO, WARN, ERROR.
	// Default: "INFO".
	Level string `yaml:"level"`
	// Log mode: SIMPLE, FULL.
	// Default: "FULL".
	Mode string `yaml:"mode"`
}
type HeaderOption struct {
	// Exact row number of column name definition at a worksheet.
	// Default: 1.
	Namerow int32
	// Exact row number of column type definition at a worksheet.
	// Default: 2.
	Typerow int32
	// Exact row number of column note at a worksheet.
	// Default: 3.
	Noterow int32
	// Start row number of data at a worksheet.
	// Default: 4.
	Datarow int32

	// The line number of column name definition in a cell.
	// Value 0 means the whole cell.
	// Default: 0.
	Nameline int32
	// The line number of column type definition in a cell.
	// Value 0 means the whole cell.
	// Default: 0.
	Typeline int32
}

type InputOption struct {
	// Input options for generating proto files.
	Proto *InputProtoOption `yaml:"proto"`
	// Input options for generating conf files.
	Conf *InputConfOption `yaml:"conf"`
}

type OutputOption struct {
	// Output options for generating proto files.
	Proto *OutputProtoOption `yaml:"proto"`
	// Output options for generating conf files.
	Conf *OutputConfOption `yaml:"conf"`
}

// Input options for generating proto files. Only for protogen.
type InputProtoOption struct {
	// Header options of worksheet.
	Header *HeaderOption `yaml:"header"`
	// The proto paths are used to search for dependencies that are referenced in import
	// statements in proto source files. If no import paths are provided then
	// "." (current directory) is assumed to be the only import path.
	// Default: nil.
	ProtoPaths []string `yaml:"protoPaths"`
	// The enums and messages in ImportedProtoFiles can be used in Excel/CSV/XML as
	// common types.
	// Default: nil.
	ImportedProtoFiles []string `yaml:"importedProtoFiles"`
	// Specify input file formats.
	// Note: recognize all formats (Excel/CSV/XML) if not set (value is nil).
	// Default: nil.
	Formats []format.Format `yaml:"formats"`
	// Specify only these subdirs (relative to input dir) to be processed.
	Subdirs []string `yaml:"subdirs"`
	// Specify rewrite subdir path (relative to input dir).
	// Default: nil.
	SubdirRewrites map[string]string `yaml:"subdirRewrites"`
	// Follow the symbolic links when traversing directories recursively.
	// WARN: be careful to use this option, it may lead to infinite loop.
	// Default: false.
	FollowSymlink bool `yaml:"followSymlink"`
}

// Input options for generating conf files. Only for confgen.
type InputConfOption struct {
	// The proto paths are used to search for dependencies that are referenced in import
	// statements in proto source files. If no import paths are provided then
	// "." (current directory) is assumed to be the only import path.
	//
	// Default: nil.
	ProtoPaths []string `yaml:"protoPaths"`
	// The files to be parsed to generate configurations.
	//
	// NOTE: Glob patterns is supported, which can specify sets 
	// of filenames with wildcard characters.
	//
	// Default: nil.
	ProtoFiles []string `yaml:"protoFiles"`
	// The files not to be parsed to generate configurations.
	//
	// NOTE: Glob patterns is supported, which can specify sets 
	// of filenames with wildcard characters.
	//
	// Default: nil.
	ExcludedProtoFiles []string `yaml:"excludedProtoFiles"`
	// Specify input file formats to be parsed.
	// Note: recognize all formats (Excel/CSV/XML) if not set (value is nil).
	// Default: nil.
	Formats []format.Format
	// Specify only these subdirs (relative to workbook name option in proto file).
	Subdirs []string
	// Specify rewrite subdir path (relative to workbook name option in proto file).
	// Default: nil.
	SubdirRewrites map[string]string `yaml:"subdirRewrites"`
}

// Output options for generating proto files. Only for protogen.
type OutputProtoOption struct {
	// Specify subdir (relative to output dir) for generated proto files.
	// Default: "".
	Subdir string `yaml:"subdir"`
	// Dir separator `/` or `\`  in filename is replaced by "__".
	// Default: false.
	FilenameWithSubdirPrefix bool `yaml:"filenameWithSubdirPrefix"`
	// Append suffix to generated proto filename.
	// Default: "".
	FilenameSuffix string `yaml:"filenameSuffix"`

	// Specify proto file options.
	// Example: go_package, csharp_namespace...
	// Default: nil.
	FileOptions map[string]string `yaml:"fileOptions"`
}

// Output options for generating conf files. Only for confgen.
type OutputConfOption struct {
	// Specify subdir (relative to output dir) for generated configuration files.
	// Default: "".
	Subdir string `yaml:"subdir"`
	// Specify generated conf file formats. If not set, it will generate all formats
	// (JSON, Text, and Wire) .
	// Default: nil.
	Formats []format.Format
	// Output pretty format, with multiline and indent.
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
	EmitUnpopulated bool `yaml:"emitUnpopulated"`
}

// Option is the functional option type.
type Option func(*Options)

// Log sets log options.
func Log(o *LogOption) Option {
	return func(opts *Options) {
		opts.Log = o
	}
}

// LocationName sets TZ location name for parsing datetime format.
func LocationName(o string) Option {
	return func(opts *Options) {
		opts.LocationName = o
	}
}

// Input sets InputOption.
func Input(o *InputOption) Option {
	return func(opts *Options) {
		opts.Input = o
	}
}

// InputProto set options for generating proto files.
func InputProto(o *InputProtoOption) Option {
	return func(opts *Options) {
		opts.Input = &InputOption{
			Proto: o,
		}
	}
}

// InputConf set options for generating conf files.
func InputConf(o *InputConfOption) Option {
	return func(opts *Options) {
		opts.Input = &InputOption{
			Conf: o,
		}
	}
}

// Output sets OutputOption.
func Output(o *OutputOption) Option {
	return func(opts *Options) {
		opts.Output = o
	}
}

// OutputProto set options for generating proto files.
func OutputProto(o *OutputProtoOption) Option {
	return func(opts *Options) {
		opts.Output = &OutputOption{
			Proto: o,
		}
	}
}

// OutputConf set options for generating conf files.
func OutputConf(o *OutputConfOption) Option {
	return func(opts *Options) {
		opts.Output = &OutputOption{
			Conf: o,
		}
	}
}

// NewDefault returns a default Options.
func NewDefault() *Options {
	return &Options{
		LocationName: "Local",
		Log: &LogOption{
			Mode:  "FULL",
			Level: "INFO",
		},
		Input: &InputOption{
			Proto: &InputProtoOption{
				Header: &HeaderOption{
					Namerow: 1,
					Typerow: 2,
					Noterow: 3,
					Datarow: 4,
				},
			},
			Conf: &InputConfOption{},
		},
		Output: &OutputOption{
			Proto: &OutputProtoOption{},
			Conf:  &OutputConfOption{},
		},
	}
}

// ParseOptions parses functional options and merge them to default Options.
func ParseOptions(setters ...Option) *Options {
	// Default Options
	opts := NewDefault()
	for _, setter := range setters {
		setter(opts)
	}
	return opts
}
