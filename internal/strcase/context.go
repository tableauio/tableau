package strcase

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

type ctxKey struct{}

type acronymRegex struct {
	Regexp      *regexp.Regexp
	Pattern     string
	Replacement string
}

type Context struct {
	acronyms map[string]*acronymRegex
}

// NewContext creates a new context with the given acronyms.
//
// Examples:
//
//   - "API": "api"
//   - "K8s": "k8s"
//   - "3D": "3d"
//   - `A(1\d{3})`: "a$l1}"
//   - `(\d)[vV](\d)`: "${1}v${2}"
func NewContext(ctx context.Context, acronyms map[string]string) context.Context {
	parsedAcronyms := make(map[string]*acronymRegex, len(acronyms))
	for pattern, replacement := range acronyms {
		parsedAcronyms[pattern] = &acronymRegex{
			Regexp:      regexp.MustCompile(pattern),
			Pattern:     pattern,
			Replacement: replacement,
		}
	}
	return context.WithValue(ctx, ctxKey{}, &Context{
		acronyms: parsedAcronyms,
	})
}

func FromContext(ctx context.Context) *Context {
	s, _ := ctx.Value(ctxKey{}).(*Context)
	return s
}

func (ctx *Context) rangeAcronym(full string, pos int) (*acronymRegex, string) {
	if ctx == nil {
		return nil, ""
	}
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
