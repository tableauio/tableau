package strcase

import (
	"strings"
)

// Converts a string to camelCase/CamelCase. The first word starting with
// initial uppercase or lowercase letter.
func (ctx *Strcase) toCamelInitCase(s string, initUppercase bool) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	n := strings.Builder{}
	n.Grow(len(s))
	upperNext := initUppercase
	prevIsUpper := false

	bytes := []byte(s)
	for i := 0; i < len(bytes); i++ {
		// treat acronyms as words, e.g.: for JSONData -> JSON is a whole word
		acronym, prefix := ctx.rangeAcronym(s, i)
		if acronym != nil {
			val := acronym.Regexp.ReplaceAllString(prefix, acronym.Replacement)
			if i > 0 || upperNext {
				val = upperFirst(val)
			} else {
				val = lowerFirst(val)
			}
			n.WriteString(val)
			i += len(prefix) - 1
			upperNext = true
			continue
		}

		v := bytes[i]
		vIsUpper := isUpper(v)
		vIsLower := isLower(v)
		if upperNext {
			if vIsLower {
				v = toUpper(v)
			}
		} else if i == 0 {
			if vIsUpper {
				v = toLower(v)
			}
		} else if prevIsUpper && vIsUpper {
			v = toLower(v)
		}
		prevIsUpper = vIsUpper

		if vIsUpper || vIsLower {
			n.WriteByte(v)
			upperNext = false
		} else if isDigit(v) {
			n.WriteByte(v)
			upperNext = true
		} else {
			upperNext = isSeparator(v)
		}
	}
	return n.String()
}

// ToCamel converts a string to CamelCase
func (ctx *Strcase) ToCamel(s string) string {
	return ctx.toCamelInitCase(s, true)
}

// ToLowerCamel converts a string to lowerCamelCase
func (ctx *Strcase) ToLowerCamel(s string) string {
	return ctx.toCamelInitCase(s, false)
}
