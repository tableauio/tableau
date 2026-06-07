// Package strcase converts strings to various cases under STYLE2024 rules
// (https://protobuf.dev/programming-guides/style/).
//
//	| Function                | Result               |
//	|-------------------------|----------------------|
//	| ToCamel(s)              | AnyKindOfString      |
//	| ToLowerCamel(s)         | anyKindOfString      |
//	| ToSnake(s)              | any_kind_of_string   |
//	| ToScreamingSnake(s)     | ANY_KIND_OF_STRING   |
//
// STYLE2024 highlights enforced (or assumed) by this package:
//
//   - Message / type names: PascalCase, no underscores. Example: "SongRequest".
//   - Field names: lower_snake_case (repeated fields use plurals). Example:
//     "song_name", "songs".
//   - Oneof names: lower_snake_case. Example: "song_id".
//   - Enum type names: PascalCase. Example: "FooBar".
//   - Enum value names: UPPER_SNAKE_CASE. They MUST start with the enum type
//     name as prefix, and the first value (zero) MUST end with "_UNSPECIFIED"
//     or "_UNKNOWN". Example: "FOO_BAR_UNSPECIFIED", "FOO_BAR_FIRST_VALUE".
//   - Underscores are only allowed in front of a letter. So "DEVICE_TIER_1"
//     is illegal, must be "DEVICE_TIER_TIER1" instead. Likewise the snake form
//     of "Tier1" is "tier1", NOT "tier_1".
//   - Acronyms are treated as ordinary words: "GetDnsRequest" / "dns_request",
//     NOT "GetDNSRequest" / "d_n_s_request".
//
// # Legacy (pre-STYLE2024) mode
//
// Existing projects whose generated proto files were produced under the
// pre-STYLE2024 algorithm can opt back into the old behavior by constructing
// the engine via [NewLegacy] (or, at the public API level, by setting
// proto.input.useLegacyNamingStyle: true in their tableau config). The flag
// is honored only by protogen — confgen always parses input under STYLE2024
// rules — and is force-disabled when proto.output.edition >= 2024.
// In legacy mode:
//
//   - Underscores ARE inserted at letter <-> digit boundaries
//     (e.g. "Tier1" -> "tier_1", "DeviceTier" / "1" -> "DEVICE_TIER_1").
//   - EnumValue does NOT inject a leading "V" for digit-led suffixes;
//     ProtoOutputOption.EnumValueWithPrefix continues to gate prefixing
//     at the call site (see options.EnumValueWithPrefix).
//   - Consecutive uppercase letters are folded to lower in camel form
//     (e.g. "HeroNTagMFcX" -> "HeroNtagMfcX").
//
// See the strcase tests (camel_test.go / snake_test.go / enum_value_test.go)
// for an exhaustive, side-by-side enumeration of every divergence.
package strcase
