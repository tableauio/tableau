package format

import "github.com/pkg/errors"

type Format int

// File format
const (
	UnknownFormat Format = iota
	JSON
	Wire
	Text
	Excel
	CSV
	XML
)

// File format extension
const (
	JSONExt  string = ".json"
	WireExt  string = ".wire"
	TextExt  string = ".text"
	ExcelExt string = ".xlsx"
	CSVExt   string = ".csv"
	XMLExt   string = ".xml"
)

func Ext2Format(ext string) (Format, error) {
	fmt := UnknownFormat
	switch ext {
	case ExcelExt:
		fmt = Excel
	case XMLExt:
		fmt = XML
	case CSVExt:
		fmt = CSV
	case JSONExt:
		fmt = JSON
	case TextExt:
		fmt = Text
	case WireExt:
		fmt = Wire
	default:
		return UnknownFormat, errors.Errorf("unknown file extension: %s", ext)
	}
	return fmt, nil
}

func Format2Ext(fmt Format) (string, error) {
	ext := ""
	switch fmt {
	case Excel:
		ext = ExcelExt
	case XML:
		ext = XMLExt
	case CSV:
		ext = CSVExt
	case JSON:
		ext = JSONExt
	case Text:
		ext = TextExt
	case Wire:
		ext = WireExt
	default:
		return "", errors.Errorf("unknown file format: %v", fmt)
	}
	return ext, nil
}

func IsValidInput(fmt Format) bool {
	switch fmt {
	case Excel, XML:
		return true
	default:
		return false
	}
}