package strcase

import "slices"

type byteType int

const (
	Other byteType = iota
	Upper
	Lower
	Separator
	Digit
)

func getType(b byte) byteType {
	switch {
	case b >= 'A' && b <= 'Z':
		return Upper
	case b >= 'a' && b <= 'z':
		return Lower
	case b >= '0' && b <= '9':
		return Digit
	case b == ' ' || b == '_' || b == '-' || b == '.':
		return Separator
	default:
		return Other
	}
}

func belong(b byte, types ...byteType) bool {
	t := getType(b)
	return slices.Contains(types, t)
}

// isIdentifier checks whether b belongs Upper, Lower, or Digit.
func isIdentifier(b byte) bool {
	return belong(b, Upper, Lower, Digit)
}

func isUpper(b byte) bool {
	return getType(b) == Upper
}

func isLower(b byte) bool {
	return getType(b) == Lower
}

func isDigit(b byte) bool {
	return getType(b) == Digit
}

func isSeparator(b byte) bool {
	return getType(b) == Separator
}
