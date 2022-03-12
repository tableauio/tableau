package format

type Format int

// File format
const (
	UnknownFormat Format = iota
	// input formats below
	Excel
	CSV
	XML
	// output formats below
	JSON
	Wire
	Text
)

// File format extension
const (
	UnknownExt string = ".unknown"
	JSONExt    string = ".json"
	WireExt    string = ".wire"
	TextExt    string = ".text"
	ExcelExt   string = ".xlsx"
	CSVExt     string = ".csv"
	XMLExt     string = ".xml"
)

func Ext2Format(ext string) Format {
	switch ext {
	case ExcelExt:
		return Excel
	case CSVExt:
		return CSV
	case XMLExt:
		return XML
	case JSONExt:
		return JSON
	case TextExt:
		return Text
	case WireExt:
		return Wire
	default:
		return UnknownFormat
	}
}

func Format2Ext(fmt Format) string {
	switch fmt {
	case Excel:
		return ExcelExt
	case CSV:
		return CSVExt
	case XML:
		return XMLExt
	case JSON:
		return JSONExt
	case Text:
		return TextExt
	case Wire:
		return WireExt
	default:
		return UnknownExt
	}
}

var InputFormats = []Format{Excel, CSV, XML}
var OutputFormats = []Format{JSON, Wire, Text}

func IsInputFormat(fmt Format) bool {
	for _, f := range InputFormats {
		if f == fmt {
			return true
		}
	}
	return false
}

func Amongst(fmt Format, formats []Format) bool {
	var found bool
	for _, f := range formats {
		if f == fmt {
			found = true
			break
		}
	}
	return found
}

// FilterInput checks if this input format need to be converted.
func FilterInput(inputFormat Format, allowedInputFormats []Format) bool {
	if !IsInputFormat(inputFormat) {
		return false
	}

	if allowedInputFormats == nil || Amongst(inputFormat, allowedInputFormats) {
		return true
	}
	return false
}
