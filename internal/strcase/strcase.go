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

type Strcase struct {
	acronyms map[string]*acronymRegex
}

// New creates a new Strcase with the given acronyms.
//
// Examples:
//
//   - "API": "api"
//   - "K8s": "k8s"
//   - "3D": "3d"
//   - `A(1\d{3})`: "a$l1}"
//   - `(\d)[vV](\d)`: "${1}v${2}"
func New(acronyms map[string]string) *Strcase {
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
	}
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
