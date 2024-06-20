package format

import "path/filepath"

type Format string

// File format
const (
	UnknownFormat Format = "unknown"
	// input formats
	Excel Format = "xlsx"
	CSV   Format = "csv"
	XML   Format = "xml"
	YAML  Format = "yaml"
	// output formats
	JSON Format = "json"
	Bin  Format = "bin"
	Text Format = "txt"
)

// File format extension
const (
	UnknownExt string = ".unknown"
	// input formats
	ExcelExt string = ".xlsx"
	CSVExt   string = ".csv"
	XMLExt   string = ".xml"
	YAMLExt  string = ".yaml"
	// output formats
	JSONExt string = ".json"
	BinExt  string = ".bin"
	TextExt string = ".txt"
)

// GetFormat returns the file's format by filename extension.
func GetFormat(filename string) Format {
	return Ext2Format(filepath.Ext(filename))
}

func Ext2Format(ext string) Format {
	switch ext {
	case ExcelExt:
		return Excel
	case CSVExt:
		return CSV
	case XMLExt:
		return XML
	case YAMLExt:
		return YAML
	case JSONExt:
		return JSON
	case BinExt:
		return Bin
	case TextExt:
		return Text
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
	case YAML:
		return YAMLExt
	case JSON:
		return JSONExt
	case Bin:
		return BinExt
	case Text:
		return TextExt
	default:
		return UnknownExt
	}
}

var InputFormats = []Format{Excel, CSV, XML, YAML}
var OutputFormats = []Format{JSON, Bin, Text}

var inputDocumentFormats = map[Format]bool{
	YAML: true,
	// XML: true, // TODO: including xml
}

// IsInputFormat checks whether the fmt belongs to [InputFormats], such as Excel.
func IsInputFormat(fmt Format) bool {
	for _, f := range InputFormats {
		if f == fmt {
			return true
		}
	}
	return false
}

// IsInputDocumentFormat checks whether the fmt belongs to input document
// formats, such as yaml.
func IsInputDocumentFormat(fmt Format) bool {
	return inputDocumentFormats[fmt]
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

	if len(allowedInputFormats) == 0 || Amongst(inputFormat, allowedInputFormats) {
		return true
	}
	return false
}
