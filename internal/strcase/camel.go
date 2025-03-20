package strcase

import (
	"strings"
)

// Converts a string to camelCase/CamelCase. The first word starting with
// initial uppercase or lowercase letter.
func toCamelInitCase(s string, initUppercase bool) string {
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
		acronymFound := false
		uppercaseAcronym.Range(func(key, value any) bool {
			remain := string(bytes[i:])
			if !strings.HasPrefix(remain, key.(string)) {
				return true
			}
			val := value.(string)
			if i > 0 || upperNext {
				val = upperFirst(val)
			} else {
				val = lowerFirst(val)
			}
			n.WriteString(val)
			i += len(key.(string)) - 1
			upperNext = true
			acronymFound = true
			return false
		})
		if acronymFound {
			continue
		}

		uppercaseAcronymRegexes.Range(func(_, re any) bool {
			remain := string(bytes[i:])
			regex := re.(*AcronymRegex)
			matches := regex.Regexp.FindStringSubmatch(remain)
			if len(matches) == 0 {
				return true
			}
			key := matches[0]
			val := regex.Regexp.ReplaceAllString(key, regex.Replacement)
			if i > 0 || upperNext {
				val = upperFirst(val)
			} else {
				val = lowerFirst(val)
			}
			n.WriteString(val)
			i += len(key) - 1
			upperNext = true
			acronymFound = true
			return false
		})
		if acronymFound {
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
func ToCamel(s string) string {
	return toCamelInitCase(s, true)
}

// ToLowerCamel converts a string to lowerCamelCase
func ToLowerCamel(s string) string {
	return toCamelInitCase(s, false)
}
