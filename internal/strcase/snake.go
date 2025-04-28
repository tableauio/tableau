package strcase

import (
	"strings"
)

// ToSnake converts a string to snake_case
func (ctx *ContextData) ToSnake(s string) string {
	return ctx.ToDelimited(s, '_')
}

func (ctx *ContextData) ToSnakeWithIgnore(s string, ignore string) string {
	return ctx.ToScreamingDelimited(s, '_', ignore, false)
}

// ToScreamingSnake converts a string to SCREAMING_SNAKE_CASE
func (ctx *ContextData) ToScreamingSnake(s string) string {
	return ctx.ToScreamingDelimited(s, '_', "", true)
}

// ToKebab converts a string to kebab-case
func (ctx *ContextData) ToKebab(s string) string {
	return ctx.ToDelimited(s, '-')
}

// ToScreamingKebab converts a string to SCREAMING-KEBAB-CASE
func (ctx *ContextData) ToScreamingKebab(s string) string {
	return ctx.ToScreamingDelimited(s, '-', "", true)
}

// ToDelimited converts a string to delimited.snake.case
// (in this case `delimiter = '.'`)
func (ctx *ContextData) ToDelimited(s string, delimiter uint8) string {
	return ctx.ToScreamingDelimited(s, delimiter, "", false)
}

// ToScreamingDelimited converts a string to SCREAMING.DELIMITED.SNAKE.CASE
// (in this case `delimiter = '.'; screaming = true`)
// or delimited.snake.case
// (in this case `delimiter = '.'; screaming = false`)
func (ctx *ContextData) ToScreamingDelimited(s string, delimiter uint8, ignore string, screaming bool) string {
	n := strings.Builder{}
	n.Grow(len(s) + 2) // nominal 2 bytes of extra space for inserted delimiters

	s = strings.TrimSpace(s)
	bytes := []byte(s)
	for i := 0; i < len(bytes); i++ {
		// treat acronyms as words, e.g.: for JSONData -> JSON is a whole word
		acronym, prefix := ctx.rangeAcronym(s, i)
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
