package strcase

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var (
	acronyms = sync.Map{} // map[string]*acronymRegex
)

type acronymRegex struct {
	Regexp      *regexp.Regexp
	Pattern     string
	Replacement string
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
func ConfigureAcronym(pattern, replacement string) {
	acronyms.Store(pattern, &acronymRegex{
		Regexp:      regexp.MustCompile(pattern),
		Pattern:     pattern,
		Replacement: replacement,
	})
}

func rangeAcronym(full string, pos int) (*acronymRegex, string) {
	var (
		acronym *acronymRegex
		prefix  string
	)
	acronyms.Range(func(_, re any) bool {
		regex := re.(*acronymRegex)
		if strings.HasPrefix(regex.Pattern, "^") && pos != 0 {
			// no need to match if current position is not the start of the string
			return true
		}
		remain := full[pos:]
		matches := regex.Regexp.FindStringSubmatch(remain)
		if len(matches) == 0 {
			return true
		}
		if !strings.HasPrefix(remain, matches[0]) {
			// not current position
			return true
		}
		if acronym != nil {
			panic(fmt.Sprintf("name %s (current remain %s) match multiple patterns: %s and %s",
				full, remain, (*acronym).Pattern, regex.Pattern))
		}
		acronym = regex
		prefix = matches[0]
		return true
	})
	return acronym, prefix
}
