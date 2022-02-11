package printer

import "strings"

// Indent is a printer that indents the each depth two spaces "  ".
func Indent(depth int) string {
	return strings.Repeat("  ", depth)
}
