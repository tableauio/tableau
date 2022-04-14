package options

import "github.com/tableauio/tableau/format"

// Options is the wrapper of tableau params.
// Options follow the design of Functional Options (https://github.com/tmrts/go-patterns/blob/master/idiom/functional-options.md).
type Options struct {
	// Location represents the collection of time offsets in use in a geographical area.
	// Default: "Asia/Shanghai".
	LocationName string
	LogLevel     string        // Log level: debug, info, warn, error. Default: "info".
	Header       *HeaderOption // Header options of worksheet file.
	Input        *InputOption  // Input options.
	Output       *OutputOption // Output options.
	ImportPaths  []string      // Import paths. Default: nil.
	Imports      []string      // Import common proto files. Default: nil.
	ProtoFiles   []string      // To be parsed proto files. Default: nil.
	Workbook     string        // Workbook filename. Default: "".
	Worksheet    string        // Worksheet name. Default: "".
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
	// Only for protogen: input file formats.
	// Note: recognize all formats (Excel, CSV, and XML) if not set (value is nil).
	// Default: nil.
	Formats []format.Format
	// - For protogen, specify only these subdirs (relative to input dir) to be processed.
	// - For confgen, specify only these subdirs (relative to workbook name option in .proto file).
	Subdirs []string
	// - For protogen, rewrite subdir path (relative to input dir).
	// - For confgen, rewrite subdir path (relative to workbook name option in .proto file).
	SubdirRewrites map[string]string
}

type OutputOption struct {
	// Only for protogen: dir separator `/` or `\`  in filename is replaced by "__".
	// Default: true.
	FilenameWithSubdirPrefix bool
	// Only for protogen: append the suffix to filename.
	// Default: "".
	FilenameSuffix string
	// Only for confgen: output filename as snake_case, default is CamelCase as the protobuf message name.
	// Default: false.
	FilenameAsSnakeCase bool
	// Only for confgen: output file formats.
	// Note: Output all formats (JSON, Text, and Wire) if not set (value is nil).
	// Default: nil.
	Formats []format.Format
	// Only for confgen: output pretty format, with multiline and indent.
	// Default: true.
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
	// Default: true.
	EmitUnpopulated bool
}

// Option is the functional option type.
type Option func(*Options)

// Header sets HeaderOption.
func Header(o *HeaderOption) Option {
	return func(opts *Options) {
		opts.Header = o
	}
}

// Output sets OutputOption.
func Output(o *OutputOption) Option {
	return func(opts *Options) {
		opts.Output = o
	}
}

// Input sets InputOption.
func Input(o *InputOption) Option {
	return func(opts *Options) {
		opts.Input = o
	}
}

// InputFormats sets input formats.
func InputFormats(formats ...format.Format) Option {
	return func(opts *Options) {
		opts.Input.Formats = formats
	}
}

// InputSubdirs sets input subdirs.
func InputSubdirs(subdirs ...string) Option {
	return func(opts *Options) {
		opts.Input.Subdirs = subdirs
	}
}

// LocationName sets LocationName.
func LocationName(o string) Option {
	return func(opts *Options) {
		opts.LocationName = o
	}
}

// LogLevel sets LogLevel.
func LogLevel(level string) Option {
	return func(opts *Options) {
		opts.LogLevel = level
	}
}

// ImportPaths specifies the paths used to search for dependencies that are
// referenced in import statements in proto source files. If no import paths
// are provided then "." (current directory) is assumed to be the only import
// path.
func ImportPaths(importPaths ...string) Option {
	return func(opts *Options) {
		opts.ImportPaths = importPaths
	}
}

// Imports sets common proto files to be imported.
func Imports(imports ...string) Option {
	return func(opts *Options) {
		opts.Imports = imports
	}
}

// ProtoFiles sets proto files to be parsed.
func ProtoFiles(protoFiles ...string) Option {
	return func(opts *Options) {
		opts.ProtoFiles = protoFiles
	}
}

// Workbook sets workbook filename.
func Workbook(wb string) Option {
	return func(opts *Options) {
		opts.Workbook = wb
	}
}

// Worksheet sets worksheet name.
func Worksheet(ws string) Option {
	return func(opts *Options) {
		opts.Worksheet = ws
	}
}

// OutputFormats sets output formats.
func OutputFormats(formats ...format.Format) Option {
	return func(opts *Options) {
		opts.Output.Formats = formats
	}
}

// NewDefault returns a default Options.
func NewDefault() *Options {
	return &Options{
		LocationName: "Asia/Shanghai",
		LogLevel:     "info",

		Header: &HeaderOption{
			Namerow: 1,
			Typerow: 2,
			Noterow: 3,
			Datarow: 4,
		},
		Output: &OutputOption{
			FilenameWithSubdirPrefix: true,
			FilenameAsSnakeCase:      false,
			Formats:                  nil,
			Pretty:                   true,
			EmitUnpopulated:          true,
		},
		Input: &InputOption{
			Formats: nil,
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
