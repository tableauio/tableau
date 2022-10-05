package localizer

import (
	"fmt"

	"github.com/tableauio/tableau/internal/localizer/i18n"
	"golang.org/x/text/language"
)

var Default *Localizer

func init() {
	// init default language as English.
	err := setLang(language.English)
	if err != nil {
		panic(err)
	}
}

// SetLang sets the default language
func SetLang(defaultLang string) error {
	tag, err := language.Parse(defaultLang)
	if err != nil {
		return err
	}
	return setLang(tag)
}

// setLang sets the default language
func setLang(defaultLang language.Tag) error {
	bundle := i18n.NewBundle(defaultLang)
	if bundle == nil {
		return fmt.Errorf("bundle of lang %s not found", defaultLang)
	}
	Default = NewLocalizer(bundle)
	return nil
}

type Localizer struct {
	*i18n.Bundle
}

// NewLocalizer returns a new Localizer that looks up messages in the bundle
// according to the language preferences in langs.
//
// TODO: support language preferences in langs.
func NewLocalizer(bundle *i18n.Bundle, langs ...string) *Localizer {
	return &Localizer{bundle}
}
