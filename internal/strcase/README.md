# strcase

> NOTE: we learned a lot from package [strcase](https://github.com/iancoleman/strcase)

strcase is a go package for converting string case to various cases (e.g. [snake case](https://en.wikipedia.org/wiki/Snake_case) or [PascalCase](https://en.wikipedia.org/wiki/Camel_case)) under [STYLE2024](https://protobuf.dev/programming-guides/style/) rules.

## Example

```go
s := "AnyKind of_string"
```

| Function              | Result               |
| --------------------- | -------------------- |
| `ToCamel(s)`          | `AnyKindOfString`    |
| `ToLowerCamel(s)`     | `anyKindOfString`    |
| `ToSnake(s)`          | `any_kind_of_string` |
| `ToScreamingSnake(s)` | `ANY_KIND_OF_STRING` |

## STYLE2024 highlights

- An underscore is only allowed in front of a letter; therefore no
  underscore is inserted at letter <-> digit boundaries
  (e.g. `Tier1` -> `tier1`, NOT `tier_1`).
- Acronyms are treated as ordinary words
  (e.g. `JSONData` -> `json_data`, `userID` -> `user_id`).

## Custom Acronyms

Sometimes, text may contain specific acronyms which need to be handled in a certain way.

```go
// For "WebAPIV3Spec":
//  - ToCamel: WebApiv3Spec
//  - ToSnake: web_apiv3_spec
strcase.New(map[string]string{"APIV3": "apiv3"})
```

## Legacy (pre-STYLE2024) Mode

Existing projects whose generated proto files were produced under the old
algorithm can opt back into it by constructing the engine with `NewLegacy`
instead of `New`. At the public tableau API level this is exposed as
`useLegacyNamingStyle: true` under the `proto.input` section of the user
`config.yaml`.

The flag is honored ONLY by protogen — confgen always parses input under
STYLE2024 rules. It is also force-disabled when the requested edition is
>= 2024 (`proto.output.edition: 2024`), because edition 2024 itself
mandates the STYLE2024 naming rules.

Behavioral differences vs STYLE2024:

- Underscores ARE inserted at letter <-> digit boundaries
  (e.g. `Tier1` -> `tier_1`, `DeviceTier`/`1` -> `DEVICE_TIER_1`).
- `EnumValue` does NOT inject a leading `V` for digit-led suffixes.
  `ProtoOutputOption.EnumValueWithPrefix` continues to gate prefixing at the
  call site under legacy mode.
- Consecutive uppercase letters are folded to lower in camel form
  (e.g. `HeroNTagMFcX` -> `HeroNtagMfcX`).

See the strcase tests for an exhaustive, side-by-side enumeration of every
divergence.
