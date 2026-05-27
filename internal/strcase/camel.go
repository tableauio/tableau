package strcase

import (
	"strings"
)

// toCamelCase converts a string to PascalCase (UpperCamelCase) using a
// chunk-based rule set. The result is STYLE2024-compliant: digits stay
// attached to the adjacent letter run (e.g. "Tier1" -> "Tier1", not
// "Tier_1"), and acronyms are treated as ordinary words.
//
// The input is first split into chunks by separator characters (space /
// underscore '_' / hyphen '-' / dot '.'). Each chunk is then transformed
// independently according to these three rules (in order):
//
//  1. If the chunk is composed exclusively of upper-case letters and / or
//     digits AND contains at least one letter (i.e. it matches
//     `[A-Z0-9]*[A-Z][A-Z0-9]*`, the typical SCREAMING_SNAKE token),
//     it is converted to "first letter upper, the rest lower".
//     Examples:
//
//     "PVP"  -> "Pvp"
//     "PVE"  -> "Pve"
//     "DATA" -> "Data"
//     "ID"   -> "Id"
//
//  2. Otherwise, if the chunk's first character is a lower-case letter,
//     only its first character is upper-cased; the remaining characters
//     are kept untouched. Examples:
//
//     "test"   -> "Test"
//     "fooBar" -> "FooBar"
//     "case"   -> "Case"
//
//  3. Otherwise the chunk is left untouched. Examples:
//
//     "HeroNTagMFcX"  -> "HeroNTagMFcX"
//     "TestCase"      -> "TestCase"
//     "123"           -> "123"
//
// Chunks are then concatenated.
//
// Acronyms (registered via New) take precedence inside every chunk: at
// each byte position we first try to match an acronym; if one matches,
// its registered replacement (with its first character upper-cased) is
// emitted and we resume processing right after the matched prefix. The
// chunk's remaining contiguous non-acronym slices are each treated as
// independent sub-chunks under rules 1-3 above.
//
// Combined examples:
//
//	"PVP"                 -> "Pvp"
//	"PVE_DATA"            -> "PveData"
//	"HeroNTagMFcX_SCORE"  -> "HeroNTagMFcXScore"
//	"test_case"           -> "TestCase"
//	"foo-bar"             -> "FooBar"
//	"CONSTANT_CASE"       -> "ConstantCase"
func (ctx *Strcase) toCamelCase(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	out := strings.Builder{}
	out.Grow(len(s))

	bytes := []byte(s)
	chunkStart := -1 // -1 means "not currently inside a chunk"
	flushChunk := func(end int) {
		if chunkStart < 0 {
			return
		}
		chunk := s[chunkStart:end]
		chunkStart = -1
		if chunk == "" {
			return
		}
		out.WriteString(ctx.transformCamelChunk(chunk))
	}

	for i := 0; i < len(bytes); i++ {
		v := bytes[i]
		if isSeparator(v) {
			flushChunk(i)
			continue
		}
		if chunkStart < 0 {
			chunkStart = i
		}
	}
	flushChunk(len(bytes))

	result := out.String()
	if result == "" {
		return result
	}
	return upperFirst(result)
}

// transformCamelChunk applies the per-chunk transformation rules
// described in toCamelCase to a single non-empty chunk that does
// NOT contain separator bytes. Acronym handling is interleaved with the
// rule application: the chunk is walked left to right; whenever an
// acronym matches at the current position, its registered replacement
// (with its first character upper-cased) is emitted and walking
// resumes after the matched prefix. The maximal contiguous slices of
// the chunk that are NOT consumed by acronyms are themselves treated
// as independent sub-chunks and run through transformPlainChunk, which
// implements rules 1-3.
func (ctx *Strcase) transformCamelChunk(chunk string) string {
	if chunk == "" {
		return chunk
	}

	out := strings.Builder{}
	out.Grow(len(chunk))

	plainStart := 0
	flushPlain := func(end int) {
		if end <= plainStart {
			return
		}
		out.WriteString(transformPlainChunk(chunk[plainStart:end]))
	}

	for i := 0; i < len(chunk); {
		acronym, prefix := ctx.rangeAcronym(chunk, i)
		if acronym == nil {
			i++
			continue
		}
		flushPlain(i)
		val := acronym.Regexp.ReplaceAllString(prefix, acronym.Replacement)
		// Each acronym replacement starts a new sub-word, so its
		// first character is upper-cased.
		val = upperFirst(val)
		out.WriteString(val)
		i += len(prefix)
		plainStart = i
	}
	flushPlain(len(chunk))

	return out.String()
}

// transformPlainChunk applies rules 1-3 from toCamelCase to a chunk
// slice that has no acronym matches and no separators inside it.
func transformPlainChunk(chunk string) string {
	if chunk == "" {
		return chunk
	}
	// Rule 1: all upper letters and/or digits, with at least one letter
	// -> first letter upper, the rest lower.
	if isAllUpperOrDigitWithLetter(chunk) {
		return upperFirst(strings.ToLower(chunk))
	}
	// Rule 2: first character is a lower-case letter -> upper-case it
	// only; keep the remaining characters untouched.
	if isLower(chunk[0]) {
		return upperFirst(chunk)
	}
	// Rule 3: leave untouched.
	return chunk
}

// isAllUpperOrDigitWithLetter reports whether s is composed exclusively
// of ASCII upper-case letters and ASCII digits, AND contains at least
// one letter. An all-digit string returns false (nothing to transform).
func isAllUpperOrDigitWithLetter(s string) bool {
	hasLetter := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case isUpper(c):
			hasLetter = true
		case isDigit(c):
			// allowed
		default:
			return false
		}
	}
	return hasLetter
}

// ToCamel converts a string to PascalCase (UpperCamelCase).
//
// Under STYLE2024 rules (default):
//
//	"any_kind_of_string"  -> "AnyKindOfString"
//	"PVE_DATA"            -> "PveData"
//	"Tier1"               -> "Tier1"
//	"HeroNTagMFcX_SCORE"  -> "HeroNTagMFcXScore"
//
// Under legacy rules (NewLegacy):
//
//	"any_kind_of_string"  -> "AnyKindOfString"
//	"Tier1"               -> "Tier1"
//	"numbers2And55with000"-> "Numbers2And55With000"   // capital W
func (ctx *Strcase) ToCamel(s string) string {
	if ctx.legacy {
		return ctx.toCamelInitCaseLegacy(s, true)
	}
	return ctx.toCamelCase(s)
}

// ToLowerCamel converts a string to lowerCamelCase.
//
// Under STYLE2024 rules (default), it is derived from ToCamel by
// lower-casing the first character of the PascalCase result.
//
// Under legacy rules, the byte-level walker emits the result directly.
//
// Examples:
//
//	"any_kind_of_string"  -> "anyKindOfString"
//	"PVE_DATA"            -> "pveData"
//	"Tier1"               -> "tier1"
//	"HeroNTagMFcX_SCORE"  -> "heroNTagMFcXScore"
func (ctx *Strcase) ToLowerCamel(s string) string {
	if ctx.legacy {
		return ctx.toCamelInitCaseLegacy(s, false)
	}
	return lowerFirst(ctx.toCamelCase(s))
}
