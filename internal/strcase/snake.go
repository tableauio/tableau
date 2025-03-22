package strcase

import (
	"strings"
)

// ToSnake converts a string to snake_case
func (a Acronyms) ToSnake(s string) string {
	return a.ToDelimited(s, '_')
}

func (a Acronyms) ToSnakeWithIgnore(s string, ignore string) string {
	return a.ToScreamingDelimited(s, '_', ignore, false)
}

// ToScreamingSnake converts a string to SCREAMING_SNAKE_CASE
func (a Acronyms) ToScreamingSnake(s string) string {
	return a.ToScreamingDelimited(s, '_', "", true)
}

// ToKebab converts a string to kebab-case
func (a Acronyms) ToKebab(s string) string {
	return a.ToDelimited(s, '-')
}

// ToScreamingKebab converts a string to SCREAMING-KEBAB-CASE
func (a Acronyms) ToScreamingKebab(s string) string {
	return a.ToScreamingDelimited(s, '-', "", true)
}

// ToDelimited converts a string to delimited.snake.case
// (in this case `delimiter = '.'`)
func (a Acronyms) ToDelimited(s string, delimiter uint8) string {
	return a.ToScreamingDelimited(s, delimiter, "", false)
}

// ToScreamingDelimited converts a string to SCREAMING.DELIMITED.SNAKE.CASE
// (in this case `delimiter = '.'; screaming = true`)
// or delimited.snake.case
// (in this case `delimiter = '.'; screaming = false`)
func (a Acronyms) ToScreamingDelimited(s string, delimiter uint8, ignore string, screaming bool) string {
	n := strings.Builder{}
	n.Grow(len(s) + 2) // nominal 2 bytes of extra space for inserted delimiters

	s = strings.TrimSpace(s)
	bytes := []byte(s)
	for i := 0; i < len(bytes); i++ {
		// treat acronyms as words, e.g.: for JSONData -> JSON is a whole word
		acronym, prefix := a.rangeAcronym(s, i)
		if acronym != nil {
			val := acronym.Regexp.ReplaceAllString(prefix, acronym.Replacement)
			if screaming {
				n.WriteString(strings.ToUpper(val))
			} else {
				n.WriteString(val)
			}
			i += len(prefix) - 1
			if i+1 < len(bytes) {
				next := bytes[i+1]
				if belong(next, Upper, Lower, Digit) && !strings.ContainsAny(string(next), ignore) {
					n.WriteByte(delimiter)
				}
			}
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
