package strcase

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var (
	uppercaseAcronym = sync.Map{} // map[string]*acronymRegex
	prefixAcronym    = sync.Map{} // map[string]*acronymRegex
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
	if !strings.HasPrefix(pattern, "^") {
		uppercaseAcronym.Store(pattern, &acronymRegex{
			Regexp:      regexp.MustCompile("^" + pattern),
			Pattern:     pattern,
			Replacement: replacement,
		})
	} else {
		prefixAcronym.Store(pattern, &acronymRegex{
			Regexp:      regexp.MustCompile(pattern),
			Pattern:     pattern,
			Replacement: replacement,
		})
	}
}

func rangeAcronym(full, remain string, acronym **acronymRegex, prefix *string) func(any, any) bool {
	return func(_, re any) bool {
		regex := re.(*acronymRegex)
		matches := regex.Regexp.FindStringSubmatch(remain)
		if len(matches) == 0 {
			return true
		}
		if *acronym != nil {
			panic(fmt.Sprintf("name %s (current remain %s) match multiple patterns: %s and %s",
				full, remain, (*acronym).Pattern, regex.Pattern))
		}
		*acronym = regex
		*prefix = matches[0]
		return true
	}
}
