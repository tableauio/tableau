package strcase

import (
	"fmt"
	"regexp"
	"strings"
)

type acronymRegex struct {
	Regexp      *regexp.Regexp
	Pattern     string
	Replacement string
}

// Strcase is the case conversion engine. It supports two naming styles:
//
//   - Default (STYLE2024): follows the Protobuf STYLE2024 guide
//     (https://protobuf.dev/programming-guides/style/). See package doc.
//   - Legacy: the conversion behavior shipped before STYLE2024 was adopted.
//     Mainly retained so that existing projects whose generated proto / conf
//     files were produced under the old rules can keep regenerating without
//     surprise renames.
//
// All public methods (ToCamel, ToLowerCamel, ToSnake, ToScreamingSnake,
// EnumValue, ...) honor the configured style; the call sites do not need to
// know which style is in effect.
type Strcase struct {
	acronyms map[string]*acronymRegex
	// legacy selects the pre-STYLE2024 conversion algorithm. When false
	// (default) STYLE2024 rules apply.
	legacy bool
}

// New creates a new Strcase with the given acronyms. The returned instance
// uses STYLE2024 rules (legacy == false).
//
// Acronym examples:
//
//   - "API": "api"
//   - "K8s": "k8s"
//   - "3D": "3d"
//   - `A(1\d{3})`: "a${1}"
//   - `(\d)[vV](\d)`: "${1}v${2}"
func New(acronyms map[string]string) *Strcase {
	return newStrcase(acronyms, false)
}

// NewLegacy creates a new Strcase that uses the legacy (pre-STYLE2024)
// conversion algorithm. See package doc for the behavioral differences.
func NewLegacy(acronyms map[string]string) *Strcase {
	return newStrcase(acronyms, true)
}

func newStrcase(acronyms map[string]string, legacy bool) *Strcase {
	parsedAcronyms := make(map[string]*acronymRegex, len(acronyms))
	for pattern, replacement := range acronyms {
		parsedAcronyms[pattern] = &acronymRegex{
			Regexp:      regexp.MustCompile(pattern),
			Pattern:     pattern,
			Replacement: replacement,
		}
	}
	return &Strcase{
		acronyms: parsedAcronyms,
		legacy:   legacy,
	}
}

// Legacy reports whether this Strcase uses the legacy (pre-STYLE2024)
// algorithm.
func (ctx *Strcase) Legacy() bool {
	return ctx.legacy
}

func (ctx *Strcase) rangeAcronym(full string, pos int) (*acronymRegex, string) {
	var (
		acronym *acronymRegex
		prefix  string
	)
	for _, regex := range ctx.acronyms {
		if strings.HasPrefix(regex.Pattern, "^") && pos != 0 {
			// no need to match if current position is not the start of the string
			continue
		}
		remain := full[pos:]
		matches := regex.Regexp.FindStringSubmatch(remain)
		if len(matches) == 0 {
			continue
		}
		if !strings.HasPrefix(remain, matches[0]) {
			// not current position
			continue
		}
		if acronym != nil {
			panic(fmt.Sprintf(`"%s" (remain: "%s") match multiple patterns: "%s" and "%s"`,
				full, remain, (*acronym).Pattern, regex.Pattern))
		}
		acronym = regex
		prefix = matches[0]
	}
	return acronym, prefix
}
