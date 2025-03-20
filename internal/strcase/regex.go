package strcase

import "regexp"

var uppercaseAcronymRegexes = []*AcronymRegex{}

type AcronymRegex struct {
	Regexp      *regexp.Regexp
	Replacement string
}

// ConfigureAcronymRegex allows you to add additional patterns which will be considered
// as acronyms.
//
// Examples:
//
//	ConfigureAcronymRegex("A(1[0-9]{3})", "a${1}")
func ConfigureAcronymRegex(pattern, replacement string) {
	uppercaseAcronymRegexes = append(uppercaseAcronymRegexes, &AcronymRegex{
		Regexp:      regexp.MustCompile("^" + pattern),
		Replacement: replacement,
	})
}
