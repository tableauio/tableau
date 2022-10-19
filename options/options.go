package options

import (
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/log"
)

// Options is the wrapper of tableau params.
// Options follow the design of Functional Options (https://github.com/tmrts/go-patterns/blob/master/idiom/functional-options.md).
type Options struct {
	// locale BCP 47 language tags: en, zh.
	//
	// Default: "en".
	Lang string

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

	Log *log.Options // Log options.

	Proto *ProtoOption `yaml:"proto"` // Proto generation options.
	Conf  *ConfOption  `yaml:"conf"`  // Conf generation options.
}

type HeaderOption struct {
	// Exact row number of column name definition at a worksheet.
	//
	// Default: 1.
	Namerow int32
	// Exact row number of column type definition at a worksheet.
	//
	// Default: 2.
	Typerow int32
	// Exact row number of column note at a worksheet.
	//
	// Default: 3.
	Noterow int32
	// Start row number of data at a worksheet.
	//
	// Default: 4.
	Datarow int32

	// The line number of column name definition in a cell.
	// Value 0 means the whole cell.
	//
	// Default: 0.
	Nameline int32
	// The line number of column type definition in a cell.
	// Value 0 means the whole cell.
	//
	// Default: 0.
	Typeline int32
}

// Options for generating proto files. Only for protogen.
type ProtoOption struct {
	// Input options for generating proto files.
	Input *ProtoInputOption `yaml:"input"`
	// Output options for generating proto files.
	Output *ProtoOutputOption `yaml:"output"`
}

// Input options for generating proto files.
type ProtoInputOption struct {
	// Header options of worksheet.
	Header *HeaderOption `yaml:"header"`
	// The proto paths are used to search for dependencies that are referenced in import
	// statements in proto source files. If no import paths are provided then
	// "." (current directory) is assumed to be the only import path.
	//
	// Default: nil.
	ProtoPaths []string `yaml:"protoPaths"`
	// The enums and messages in ProtoFiles can be used in Excel/CSV/XML as
	// common types.
	//
	// Default: nil.
	ProtoFiles []string `yaml:"protoFiles"`
	// Specify input file formats.
	// Note: recognize all formats (Excel/CSV/XML) if not set (value is nil).
	//
	// Default: nil.
	Formats []format.Format `yaml:"formats"`
	// Specify only these subdirs (relative to input dir) to be processed.
	//
	// Default: nil.
	Subdirs []string `yaml:"subdirs"`
	// Specify rewrite subdir path (relative to input dir).
	//
	// Default: nil.
	SubdirRewrites map[string]string `yaml:"subdirRewrites"`
	// Follow the symbolic links when traversing directories recursively.
	// WARN: be careful to use this option, it may lead to infinite loop.
	//
	// Default: false.
	FollowSymlink bool `yaml:"followSymlink"`
}

// Output options for generating proto files.
type ProtoOutputOption struct {
	// Specify subdir (relative to output dir) for generated proto files.
	//
	// Default: "".
	Subdir string `yaml:"subdir"`
	// Dir separator `/` or `\`  in filename is replaced by "__".
	//
	// Default: false.
	FilenameWithSubdirPrefix bool `yaml:"filenameWithSubdirPrefix"`
	// Append suffix to each generated proto filename.
	//
	// Default: "".
	FilenameSuffix string `yaml:"filenameSuffix"`

	// Specify proto file options.
	// Example: go_package, csharp_namespace...
	//
	// Default: nil.
	FileOptions map[string]string `yaml:"fileOptions"`
}

// Options for generating conf files. Only for confgen.
type ConfOption struct {
	// Input options for generating conf files.
	Input *ConfInputOption `yaml:"input"`
	// Output options for generating conf files.
	Output *ConfOutputOption `yaml:"output"`
}

// Input options for generating conf files.
type ConfInputOption struct {
	// The proto paths are used to search for dependencies that are referenced in import
	// statements in proto source files. If no import paths are provided then
	// "." (current directory) is assumed to be the only import path.
	//
	// Default: nil.
	ProtoPaths []string `yaml:"protoPaths"`
	// The files to be parsed to generate configurations.
	//
	// NOTE:
	//  - Recognize "*.proto" pattern if not set (value is nil).
	//  - Glob patterns are supported, which can specify sets
	//    of filenames with wildcard characters.
	//
	// Default: nil.
	ProtoFiles []string `yaml:"protoFiles"`
	// The files not to be parsed to generate configurations.
	//
	// NOTE: Glob patterns are supported, which can specify sets
	// of filenames with wildcard characters.
	//
	// Default: nil.
	ExcludedProtoFiles []string `yaml:"excludedProtoFiles"`
	// Specify input file formats to be parsed.
	// Note: recognize all formats (Excel/CSV/XML) if not set (value is nil).
	//
	// Default: nil.
	Formats []format.Format
	// Specify only these subdirs (relative to workbook name option in proto file).
	//
	// Default: nil.
	Subdirs []string
	// Specify rewrite subdir path (relative to workbook name option in proto file).
	//
	// Default: nil.
	SubdirRewrites map[string]string `yaml:"subdirRewrites"`
}

// Output options for generating conf files.
type ConfOutputOption struct {
	// Specify subdir (relative to output dir) for generated configuration files.
	//
	// Default: "".
	Subdir string `yaml:"subdir"`
	// Specify generated conf file formats. If not set, it will generate all
	// formats (JSON/Text/Bin).
	//
	// Default: nil.
	Formats []format.Format
	// Output pretty format of JSON, with multiline and indent.
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
	EmitUnpopulated bool `yaml:"emitUnpopulated"`
}

// Option is the functional option type.
type Option func(*Options)

// LocationName sets TZ location name for parsing datetime format.
func LocationName(o string) Option {
	return func(opts *Options) {
		opts.LocationName = o
	}
}

// Lang sets BCP 47 language tags: en, zh.
func Lang(o string) Option {
	return func(opts *Options) {
		opts.Lang = o
	}
}

// Log sets log options.
func Log(o *log.Options) Option {
	return func(opts *Options) {
		opts.Log = o
	}
}

// Proto sets ProtoOption.
func Proto(o *ProtoOption) Option {
	return func(opts *Options) {
		opts.Proto = o
	}
}

// Conf sets ConfOption.
func Conf(o *ConfOption) Option {
	return func(opts *Options) {
		opts.Conf = o
	}
}

// NewDefault returns a default Options.
func NewDefault() *Options {
	return &Options{
		Lang:         "en",
		LocationName: "Local",
		Log: &log.Options{
			Mode:  "SIMPLE",
			Level: "INFO",
			Sink:  "CONSOLE",
		},
		Proto: &ProtoOption{
			Input: &ProtoInputOption{
				Header: &HeaderOption{
					Namerow: 1,
					Typerow: 2,
					Noterow: 3,
					Datarow: 4,
				},
				ProtoPaths: []string{"."},
			},
			Output: &ProtoOutputOption{},
		},
		Conf: &ConfOption{
			Input: &ConfInputOption{
				ProtoPaths: []string{"."},
				ProtoFiles: []string{"*.proto"},
			},
			Output: &ConfOutputOption{
				Formats: []format.Format{format.JSON},
				Pretty:  true,
			},
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
