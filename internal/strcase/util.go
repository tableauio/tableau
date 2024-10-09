package strcase

import "unicode"

type byteType int

const (
	Other byteType = iota
	Upper
	Lower
	Separator
	Digit
)

func getType(v byte) byteType {
	switch {
	case v >= 'A' && v <= 'Z':
		return Upper
	case v >= 'a' && v <= 'z':
		return Lower
	case v >= '0' && v <= '9':
		return Digit
	case v == ' ' || v == '_' || v == '-' || v == '.':
		// space/underscore/hyphen/dot
		return Separator
	default:
		return Other
	}
}

func belong(v byte, types ...byteType) bool {
	t := getType(v)
	for _, typ := range types {
		if t == typ {
			return true
		}
	}
	return false
}

func isUpper(v byte) bool {
	return getType(v) == Upper
}

func isLower(v byte) bool {
	return getType(v) == Lower
}

func isDigit(v byte) bool {
	return getType(v) == Digit
}

func isSeparator(v byte) bool {
	return getType(v) == Separator
}

func toUpper(v byte) byte {
	return byte(unicode.ToUpper(rune(v)))
}

func toLower(v byte) byte {
	return byte(unicode.ToLower(rune(v)))
}

// upperFirst converts the first character of a string to uppercase.
func upperFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(unicode.ToUpper(rune(s[0]))) + s[1:]
}

// lowerFirst converts the first character of a string to lowercase.
func lowerFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	return string(unicode.ToLower(rune(s[0]))) + s[1:]
}
