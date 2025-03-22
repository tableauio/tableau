package strcase

import (
	"fmt"
	"regexp"
	"strings"
)

type AcronymRegex struct {
	Regexp      *regexp.Regexp
	Pattern     string
	Replacement string
}

type Acronyms map[string]*AcronymRegex

func ParseAcronyms(acronyms map[string]string) Acronyms {
	acronymsParsed := make(map[string]*AcronymRegex, len(acronyms))
	for pattern, replacement := range acronyms {
		acronymsParsed[pattern] = &AcronymRegex{
			Regexp:      regexp.MustCompile(pattern),
			Pattern:     pattern,
			Replacement: replacement,
		}
	}
	return acronymsParsed
}

// ConfigureAcronym allows you to add additional patterns which will be considered
// as acronyms.
//
// Examples:
//
//	ConfigureAcronym("API", "api")
//	ConfigureAcronym("K8s", "k8s")
//	ConfigureAcronym("3D", "3d")
//	ConfigureAcronym(`A(1\d{3})`, "a${1}")
//	ConfigureAcronym(`(\d)[vV](\d)`, "${1}v${2}")

func (a Acronyms) rangeAcronym(full string, pos int) (*AcronymRegex, string) {
	var (
		acronym *AcronymRegex
		prefix  string
	)
	for _, regex := range a {
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
