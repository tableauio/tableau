package strcase

import (
	"strings"
)

// This file holds the pre-STYLE2024 ("legacy") conversion algorithms.
// They are byte-for-byte ports of the implementation that shipped before
// the STYLE2024 refactor, kept here so users who opt in via
// useLegacyNamingStyle keep producing the exact same generated names as
// before. Behavioral differences vs the STYLE2024 algorithm:
//
//   - Underscores ARE inserted at letter <-> digit boundaries
//     (e.g. "Tier1" -> "tier_1", "1A2" -> "1_a_2").
//   - Acronyms are folded against an UPPER_lower transition rather than
//     treated as ordinary words, but in practice both algorithms produce
//     "JSONData" -> "json_data" / "userID" -> "user_id".
//   - EnumValue does NOT inject a leading "V" for digit-led suffixes
//     (legacy callers were free to produce "DEVICE_TIER_1").

// toCamelInitCaseLegacy is the legacy camel-case engine. It walks the
// input one byte at a time, replacing acronyms eagerly and inserting
// case transitions at every separator / digit boundary.
func (ctx *Strcase) toCamelInitCaseLegacy(s string, initUppercase bool) string {
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

// toScreamingDelimitedLegacy is the legacy snake/screaming-snake engine.
// It always inserts a delimiter at every case-transition boundary,
// including letter <-> digit.
func (ctx *Strcase) toScreamingDelimitedLegacy(s string, delimiter uint8, screaming bool) string {
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
				if belong(next, Upper, Lower, Digit) {
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

		if isSeparator(v) {
			// replace space/underscore/hyphen/dot with delimiter
			n.WriteByte(delimiter)
		} else {
			n.WriteByte(v)
		}
	}

	return n.String()
}

// enumValueLegacy mirrors how legacy callers built enum value names: just
// "<UPPER_SNAKE_CASE(enumName)>_<UPPER_SNAKE_CASE(value)>", with the
// already-prefixed-value idempotent behavior kept for parity with the new
// EnumValue. NOTE: legacy callers chose whether to actually prefix at the
// call site (via ProtoOutputOption.EnumValueWithPrefix); this helper
// always prefixes. Call sites that want the legacy "opt-in" behavior must
// check EnumValueWithPrefix themselves before calling EnumValue.
func (ctx *Strcase) enumValueLegacy(enumName, value string) string {
	prefix := ctx.toScreamingDelimitedLegacy(enumName, '_', true) + "_"
	v := strings.TrimSpace(value)
	if v == "" {
		return prefix
	}
	if strings.HasPrefix(v, prefix) {
		rest := strings.TrimPrefix(v, prefix)
		return prefix + ctx.toScreamingDelimitedLegacy(rest, '_', true)
	}
	return prefix + ctx.toScreamingDelimitedLegacy(v, '_', true)
}
