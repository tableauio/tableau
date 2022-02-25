package printer

import "strings"

// Indent indents each depth two spaces "  ".
func Indent(depth int) string {
	return strings.Repeat("  ", depth)
}
