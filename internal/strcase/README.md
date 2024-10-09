# strcase

> NOTE: we learned a lot from package [strcase](https://github.com/iancoleman/strcase)

strcase is a go package for converting string case to various cases (e.g. [snake case](https://en.wikipedia.org/wiki/Snake_case) or [camel case](https://en.wikipedia.org/wiki/CamelCase)) to see the full conversion table below.

## Example

```go
s := "AnyKind of_string"
```

| Function                                  | Result               |
| ----------------------------------------- | -------------------- |
| `ToSnake(s)`                              | `any_kind_of_string` |
| `ToSnakeWithIgnore(s, '.')`               | `any_kind.of_string` |
| `ToScreamingSnake(s)`                     | `ANY_KIND_OF_STRING` |
| `ToKebab(s)`                              | `any-kind-of-string` |
| `ToScreamingKebab(s)`                     | `ANY-KIND-OF-STRING` |
| `ToDelimited(s, '.')`                     | `any.kind.of.string` |
| `ToScreamingDelimited(s, '.', '', true)`  | `ANY.KIND.OF.STRING` |
| `ToScreamingDelimited(s, '.', ' ', true)` | `ANY.KIND OF.STRING` |
| `ToCamel(s)`                              | `AnyKindOfString`    |
| `ToLowerCamel(s)`                         | `anyKindOfString`    |

## Custom Acronyms

Sometimes, text may contain specific acronyms which need to be handled in a certain way.

To configure your custom acronyms globally you can use the following before running any conversion.

```go
// For "WebAPIV3Spec":
//  - ToCamel: WebApiv3Spec
//  - ToLowerCamel: webApiv3Spec
//  - ToSnake: web_apiv3_spec
strcase.ConfigureAcronym("APIV3", "apiv3")
```
