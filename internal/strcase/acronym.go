package strcase

import (
	"regexp"
	"strings"
	"sync"
)

var uppercaseAcronym = sync.Map{} // map[string]*acronymRegex

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
	compilePattern := pattern
	if !strings.HasPrefix(pattern, "^") {
		compilePattern = "^" + pattern
	}
	uppercaseAcronym.Store(pattern, &acronymRegex{
		Regexp:      regexp.MustCompile(compilePattern),
		Pattern:     pattern,
		Replacement: replacement,
	})
}
