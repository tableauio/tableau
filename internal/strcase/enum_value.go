package strcase

import (
	"strings"
)

// EnumValue returns the STYLE2024-compliant enum value name for the given
// raw value name under the given enum type name. Specifically:
//
//  1. enumName is converted to UPPER_SNAKE_CASE under STYLE2024 rules and used
//     as the prefix.
//  2. The raw value is converted to UPPER_SNAKE_CASE under STYLE2024 rules.
//  3. If the raw value (after normalization) does NOT start with a letter
//     (e.g. it starts with a digit), a leading "V" is inserted so that the
//     remainder after stripping the prefix is still a valid identifier.
//     Example: enum DeviceTier value "1" -> "DEVICE_TIER_V1".
//  4. The prefix is prepended unless the value is already prefixed.
//
// Examples:
//
//	EnumValue("DeviceTier", "Tier1")  -> "DEVICE_TIER_TIER1"
//	EnumValue("DeviceTier", "1")      -> "DEVICE_TIER_V1"
//	EnumValue("ItemType",   "EQUIP")  -> "ITEM_TYPE_EQUIP"
//	EnumValue("ItemType",   "ITEM_TYPE_EQUIP") -> "ITEM_TYPE_EQUIP"
func (ctx *Strcase) EnumValue(enumName, value string) string {
	prefix := ctx.ToScreamingSnake(enumName) + "_"
	v := strings.TrimSpace(value)
	if v == "" {
		return prefix
	}
	// If user already wrote a fully-qualified value, normalize and keep it.
	if strings.HasPrefix(v, prefix) {
		rest := strings.TrimPrefix(v, prefix)
		rest = ensureLeadingLetter(ctx.ToScreamingSnake(rest))
		return prefix + rest
	}
	norm := ensureLeadingLetter(ctx.ToScreamingSnake(v))
	return prefix + norm
}

// ensureLeadingLetter prepends "V" if s does not start with a letter, so that
// the remainder (when used as the suffix part of an enum value) is still a
// valid identifier after the enum-name prefix is stripped. STYLE2024 forbids
// names like "DEVICE_TIER_1" because the suffix after stripping is "1".
func ensureLeadingLetter(s string) string {
	if s == "" {
		return s
	}
	c := s[0]
	if isUpper(c) || isLower(c) {
		return s
	}
	return "V" + s
}
