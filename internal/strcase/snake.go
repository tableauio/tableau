package strcase

import (
	"strings"
)

// ToSnake converts a string to snake_case
func ToSnake(s string) string {
	return ToDelimited(s, '_')
}

func ToSnakeWithIgnore(s string, ignore string) string {
	return ToScreamingDelimited(s, '_', ignore, false)
}

// ToScreamingSnake converts a string to SCREAMING_SNAKE_CASE
func ToScreamingSnake(s string) string {
	return ToScreamingDelimited(s, '_', "", true)
}

// ToKebab converts a string to kebab-case
func ToKebab(s string) string {
	return ToDelimited(s, '-')
}

// ToScreamingKebab converts a string to SCREAMING-KEBAB-CASE
func ToScreamingKebab(s string) string {
	return ToScreamingDelimited(s, '-', "", true)
}

// ToDelimited converts a string to delimited.snake.case
// (in this case `delimiter = '.'`)
func ToDelimited(s string, delimiter uint8) string {
	return ToScreamingDelimited(s, delimiter, "", false)
}

// ToScreamingDelimited converts a string to SCREAMING.DELIMITED.SNAKE.CASE
// (in this case `delimiter = '.'; screaming = true`)
// or delimited.snake.case
// (in this case `delimiter = '.'; screaming = false`)
func ToScreamingDelimited(s string, delimiter uint8, ignore string, screaming bool) string {
	n := strings.Builder{}
	n.Grow(len(s) + 2) // nominal 2 bytes of extra space for inserted delimiters

	s = strings.TrimSpace(s)
	bytes := []byte(s)
	for i := 0; i < len(bytes); i++ {
		// treat acronyms as words, e.g.: for JSONData -> JSON is a whole word
		acronymFound := false
		uppercaseAcronym.Range(func(key, value any) bool {
			remain := string(bytes[i:])
			if strings.HasPrefix(remain, key.(string)) {
				n.WriteString(value.(string))
				i += len(key.(string)) - 1
				if i+1 < len(bytes) {
					next := bytes[i+1]
					if belong(next, Upper, Lower, Digit) && !strings.ContainsAny(string(next), ignore) {
						n.WriteByte(delimiter)
					}
				}
				acronymFound = true
				return false
			}
			return true
		})
		if acronymFound {
			continue
		}

		v := bytes[i]
		vIsUpper := isUpper(v)
		vIsLow := isLower(v)
		if vIsLow && screaming {
			v = toUpper(v)
		} else if vIsUpper && !screaming {
			v = toLower(v)
		}

		// treat acronyms as words, eg for JSONData -> JSON is a whole word
		if i+1 < len(s) {
			next := s[i+1]
			vIsDigit := isDigit(v)
			nextIsUpper := isUpper(next)
			nextIsLower := isLower(next)
			nextIsDigit := isDigit(next)
			// add underscore if next letter case type is changed
			if (vIsUpper && (nextIsLower || nextIsDigit)) ||
				(vIsLow && (nextIsUpper || nextIsDigit)) ||
				(vIsDigit && (nextIsUpper || nextIsLower)) {
				prevIgnore := ignore != "" && i > 0 && strings.ContainsAny(string(s[i-1]), ignore)
				if !prevIgnore {
					if vIsUpper && nextIsLower {
						if prevIsCap := i > 0 && isUpper(s[i-1]); prevIsCap {
							n.WriteByte(delimiter)
						}
					}
					n.WriteByte(v)
					if vIsLow || vIsDigit || nextIsDigit {
						n.WriteByte(delimiter)
					}
					continue
				}
			}
		}

		if isSeparator(v) && !strings.ContainsAny(string(v), ignore) {
			// replace space/underscore/hyphen/dot with delimiter
			n.WriteByte(delimiter)
		} else {
			n.WriteByte(v)
		}
	}

	return n.String()
}
