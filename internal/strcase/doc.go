// Package strcase converts strings to various cases under STYLE2024 rules
// (https://protobuf.dev/programming-guides/style/).
//
//	| Function                | Result               |
//	|-------------------------|----------------------|
//	| ToPascal(s)             | AnyKindOfString      |
//	| ToCamel(s)              | anyKindOfString      |
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
package strcase
