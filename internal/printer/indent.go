package printer

import "strings"

// Indent is a printer that indents the each depth two spaces "  ".
func Indent(depth int) string {
	return strings.Repeat("  ", depth)
}

// LetterAxis generate the corresponding column name.
// index: 0-based.
func LetterAxis(index int) string {
	var (
		colCode = ""
		key     = 'A'
		loop    = index / 26
	)
	if loop > 0 {
		colCode += LetterAxis(loop - 1)
	}
	return colCode + string(key+int32(index)%26)
}
