package strcase

import (
	"strings"
)

// ToSnake converts a string to snake_case under STYLE2024 rules.
//
// STYLE2024 highlights (https://protobuf.dev/programming-guides/style/):
//   - An underscore is only allowed in front of a letter; therefore no
//     underscore is inserted at letter <-> digit boundaries.
//   - Acronyms are treated as ordinary words (e.g. "JSONData" ->
//     "json_data", "userID" -> "user_id").
//
// Examples:
//
//	"Tier1"                -> "tier1"
//	"numbers2and55with000" -> "numbers2and55with000"
//	"AB1AB2AB3"            -> "ab1_ab2_ab3"
//	"userID"               -> "user_id"
//	"JSONData"             -> "json_data"
func (ctx *Strcase) ToSnake(s string) string {
	return ctx.toDelimited(s, '_', false)
}

// ToScreamingSnake converts a string to SCREAMING_SNAKE_CASE under
// STYLE2024 rules. Same semantics as ToSnake, but the result is upper
// cased.
//
// Examples:
//
//	"Tier1"                -> "TIER1"
//	"numbers2and55with000" -> "NUMBERS2AND55WITH000"
//	"AB1AB2AB3"            -> "AB1_AB2_AB3"
func (ctx *Strcase) ToScreamingSnake(s string) string {
	return ctx.toDelimited(s, '_', true)
}

// toDelimited is the STYLE2024-aware low-level converter shared by
// ToSnake and ToScreamingSnake.
//
// Compared with a classic snake_case converter, the key behavioral
// rule is: never insert a delimiter at a letter <-> digit boundary;
// the digit run is kept attached to the preceding (or following)
// letter run. Delimiters are inserted at:
//
//   - lower -> upper (e.g. "userId" -> "user_id")
//   - upper -> lower with previous upper (acronym boundary, e.g.
//     "JSONData" -> "json_data")
//   - digit -> upper-letter (e.g. "AB1AB2" -> "ab1_ab2")
//   - explicit separator (' ', '_', '-', '.') between tokens
//
// Acronym replacements (regex) are honored exactly like in the
// classic converter; after an acronym match we add a delimiter only
// when the next byte is a letter (NOT a digit).
//
// STYLE2024 forbids names where any underscore-separated segment
// starts with a digit (an underscore is "only allowed in front of a
// letter"). Therefore at every potential split point we suppress the
// delimiter when the next non-separator byte would be a digit; the
// digit run is glued onto the preceding segment instead. Examples:
//
//	"AB1 2CD"  -> "ab12_cd"   (NOT "ab1_2cd")
//	"foo_1bar" -> "foo1bar"   (NOT "foo_1bar")
//	"v1.2"     -> "v12"       (NOT "v1_2")
func (ctx *Strcase) toDelimited(s string, delimiter uint8, screaming bool) string {
	n := strings.Builder{}
	n.Grow(len(s) + 2)

	s = strings.TrimSpace(s)
	bytes := []byte(s)
	for i := 0; i < len(bytes); i++ {
		// treat acronyms as words, e.g. for JSONData -> JSON is a whole word
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
				// In STYLE2024 we do NOT insert a delimiter before a digit
				// (a digit-led segment would violate "underscore only in
				// front of a letter").
				if belong(next, Upper, Lower) {
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

		if i+1 < len(s) {
			next := s[i+1]
			nextIsUpper := isUpper(next)
			nextIsLower := isLower(next)
			nextIsDigit := isDigit(next)
			vIsDigit := isDigit(v)
			// STYLE2024: an underscore is only allowed in FRONT OF A LETTER.
			// Therefore we never split between a letter and an adjacent digit
			// (digit<->letter boundary stays glued together). The remaining
			// boundaries that warrant a delimiter are:
			//   - lower -> upper                  ("userId"    -> "user_id")
			//   - upper -> upper-then-lower       ("JSONData"  -> "json_data")
			//   - digit -> upper-letter           ("AB1AB2"    -> "AB1_AB2")
			if vIsUpper && nextIsLower {
				if prevIsCap := i > 0 && isUpper(s[i-1]); prevIsCap {
					n.WriteByte(delimiter)
				}
				n.WriteByte(v)
				continue
			}
			if vIsLow && nextIsUpper {
				n.WriteByte(v)
				n.WriteByte(delimiter)
				continue
			}
			if vIsDigit && nextIsUpper {
				n.WriteByte(v)
				n.WriteByte(delimiter)
				continue
			}
			// Explicit separator followed by a digit: we MUST NOT emit a
			// delimiter, because that would produce a segment starting
			// with a digit. Glue the digit run onto the previous segment
			// instead. The natural digit -> upper-letter split inside
			// that run is still honored on a later iteration, so e.g.
			// "AB1 2CD" yields "ab12_cd" (NOT "ab1_2cd" and NOT
			// "ab12cd"): both segments are letter-initial.
			if isSeparator(v) && nextIsDigit {
				continue
			}
		}

		if isSeparator(v) {
			n.WriteByte(delimiter)
		} else {
			n.WriteByte(v)
		}
	}

	return n.String()
}
