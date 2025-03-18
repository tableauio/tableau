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

	// Configure your custom acronyms. Out of the box, "ID" -> "id" is auto configured.
	//
	// For example, if you configure K8s -> k8s, then the field name in PascalCase "InK8s"
	// will be converted to snake_case "in_k8s" but not "in_k_8_s".
	Acronyms map[string]string `yaml:"acronyms"`

	Log *log.Options // Log options.

	Proto *ProtoOption `yaml:"proto"` // Proto generation options.
	Conf  *ConfOption  `yaml:"conf"`  // Conf generation options.
}

type HeaderOption struct {
	// Exact row number of column name definition at a worksheet.
	//
	// Default: 1.
	NameRow int32 `yaml:"namerow"`

	// Exact row number of column type definition at a worksheet.
	//
	// Default: 2.
	TypeRow int32 `yaml:"typerow"`
	// Exact row number of column note at a worksheet.
	//
	// Default: 3.
	NoteRow int32 `yaml:"noterow"`

	// Start row number of data at a worksheet.
	//
	// Default: 4.
	DataRow int32 `yaml:"datarow"`

	// The line number of column name definition in a cell.
	// Value 0 means the whole cell.
	//
	// Default: 0.
	NameLine int32 `yaml:"nameline"`

	// The line number of column type definition in a cell.
	// Value 0 means the whole cell.
	//
	// Default: 0.
	TypeLine int32 `yaml:"typeline"`

	// Separator for separating:
	//  - incell list elements (scalar or struct).
	//  - incell map items.
	//
	// Default: ",".
	Sep string

	// Subseparator for separating:
	//  - key-value pair of each incell map item.
	//  - struct fields of each incell struct list element.
	//
	// Default: ":".
	Subsep string
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

	// The enums and messages in ProtoFiles can be used in Excel/CSV/XML/YAML
	// as common types.
	//
	// Default: nil.
	ProtoFiles []string `yaml:"protoFiles"`

	// Specify input file formats.
	// Note: recognize all formats (Excel/CSV/XML/YAML) if not set (value is
	// nil).
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

	// Specify metasheet name. Metasheet is "@TABLEAU" if not set.
	//
	// Default: "".
	MetasheetName string `yaml:"metasheetName"`
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

	// EnumValueWithPrefix specifies whether to prepend prefix
	// "UPPER_SNAKE_CASE of EnumType" to each enum value name.
	//
	// If set, the enum value name is prepended with "ENUM_TYPE_". For example:
	// enum ItemType has a value "EQUIP", then converted to "ITEM_TYPE_EQUIP".
	// If the enum value name is already prefixed with "ENUM_TYPE_", then it will
	// not be prefixed again.
	//
	// Default: false.
	EnumValueWithPrefix bool `yaml:"enumValueWithPrefix"`
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
	// Note: recognize all formats (Excel/CSV/XML/YAML) if not set (value is
	// nil).
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

	// Whether converter will not report an error and abort if a workbook
	// is not recognized in proto files.
	//
	// Default: false.
	IgnoreUnknownWorkbook bool `yaml:"-"`
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
	EmitUnpopulated bool `yaml:"emitUnpopulated"`

	// UseProtoNames uses proto field name instead of lowerCamelCase name in JSON
	// field names.
	UseProtoNames bool `yaml:"useProtoNames"`

	// UseEnumNumbers emits enum values as numbers.
	UseEnumNumbers bool `yaml:"useEnumNumbers"`

	// Specify dry run mode:
	//  - patch: if sheet options are specified: Patch (PATCH_MERGE) and Scatter
	//
	// Default: "".
	DryRun DryRun `yaml:"dryRun"`
}

type DryRun = string

const (
	DryRunPatch DryRun = "patch"
)

const (
	DefaultNameRow = 1 // Exact row number of column name definition at a worksheet.
	DefaultTypeRow = 2 // Exact row number of column type definition at a worksheet.
	DefaultNoteRow = 3 // Exact row number of column note definition at a worksheet.
	DefaultDataRow = 4 // Start row number of data at a worksheet.
)

const (
	DefaultSep    = ","
	DefaultSubsep = ":"
)

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

// Acronyms configures your custom acronyms globally in protogen.
func Acronyms(o map[string]string) Option {
	return func(opts *Options) {
		opts.Acronyms = o
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
					NameRow: DefaultNameRow,
					TypeRow: DefaultTypeRow,
					NoteRow: DefaultNoteRow,
					DataRow: DefaultDataRow,
					Sep:     DefaultSep,
					Subsep:  DefaultSubsep,
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
