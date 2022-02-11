package options

type Format int

// file format
const (
	JSON  Format = 0
	Wire  Format = 1
	Text  Format = 2
	Excel Format = 3
	CSV   Format = 4
	XML   Format = 5
)

// file format extension

const (
	JSONExt  string = ".json"
	WireExt  string = ".wire"
	TextExt  string = ".text"
	ExcelExt string = ".xlsx"
	CSVExt   string = ".csv"
	XMLExt   string = ".xml"
)

// Options is the wrapper of tableau params.
// Options follow the design of Functional Options (https://github.com/tmrts/go-patterns/blob/master/idiom/functional-options.md).
type Options struct {
	LocationName string        // Location represents the collection of time offsets in use in a geographical area. Default is "Asia/Shanghai".
	LogLevel     string        // Log level: debug, info, warn, error
	Header       *HeaderOption // header rows of excel file.
	Output       *OutputOption // output settings.
	Input        *InputOption  // input settings.
	Imports      []string      // imported common proto file paths
	Workbook     string        // workbook path or name
	Worksheet    string        // worksheet name
}

type HeaderOption struct {
	Namerow int32
	Typerow int32
	Noterow int32
	Datarow int32

	Nameline int32
	Typeline int32
}

type OutputOption struct {
	// only for protogen generated protoconf file
	FilenameWithSubdirPrefix bool // default true, filename dir separator `/` or `\` is replaced by "__"
	FilenameSuffix           string

	// only for confgen generated JSON/Text/Wire file
	FilenameAsSnakeCase bool   // default false, output filename as snake case, default is camel case same as the protobuf message name.
	Format              Format // output pretty format, with mulitline and indent.
	Pretty              bool   // default true, output format: json, text, or wire, and default is json.
	// Output.EmitUnpopulated specifies whether to emit unpopulated fields. It does not
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
	EmitUnpopulated bool // default true
}

type InputOption struct {
	Format Format
}

// Option is the functional option type.
type Option func(*Options)

func Header(o *HeaderOption) Option {
	return func(opts *Options) {
		opts.Header = o
	}
}

func Output(o *OutputOption) Option {
	return func(opts *Options) {
		opts.Output = o
	}
}

func Input(o *InputOption) Option {
	return func(opts *Options) {
		opts.Input = o
	}
}

func LocationName(o string) Option {
	return func(opts *Options) {
		opts.LocationName = o
	}
}

func LogLevel(level string) Option {
	return func(opts *Options) {
		opts.LogLevel = level
	}
}

func Imports(imports []string) Option {
	return func(opts *Options) {
		opts.Imports = imports
	}
}

func Workbook(wb string) Option {
	return func(opts *Options) {
		opts.Workbook = wb
	}
}

func Worksheet(ws string) Option {
	return func(opts *Options) {
		opts.Worksheet = ws
	}
}
func newDefaultOptions() *Options {
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
			Format:                   JSON,
			Pretty:                   true,
			EmitUnpopulated:          true,
		},
		Input: &InputOption{
			Format: Excel,
		},
	}
}

func ParseOptions(setters ...Option) *Options {
	// Default Options
	opts := newDefaultOptions()
	for _, setter := range setters {
		setter(opts)
	}
	return opts
}

func Ext2Format(ext string) Format {
	switch ext {
	case ExcelExt:
		return Excel
	case XMLExt:
		return XML
	case CSVExt:
		return CSV
	default:
		return Excel
	}
}
