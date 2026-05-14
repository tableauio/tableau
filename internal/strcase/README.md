# strcase

> NOTE: we learned a lot from package [strcase](https://github.com/iancoleman/strcase)

strcase is a go package for converting string case to various cases (e.g. [snake case](https://en.wikipedia.org/wiki/Snake_case) or [PascalCase](https://en.wikipedia.org/wiki/Camel_case)) under [STYLE2024](https://protobuf.dev/programming-guides/style/) rules.

## Example

```go
s := "AnyKind of_string"
```

| Function              | Result               |
| --------------------- | -------------------- |
| `ToPascal(s)`         | `AnyKindOfString`    |
| `ToCamel(s)`          | `anyKindOfString`    |
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
//  - ToPascal: WebApiv3Spec
//  - ToSnake:  web_apiv3_spec
strcase.New(map[string]string{"APIV3": "apiv3"})
```
