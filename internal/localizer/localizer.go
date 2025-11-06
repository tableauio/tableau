package localizer

import (
	"fmt"

	"github.com/tableauio/tableau/internal/localizer/i18n"
	"golang.org/x/text/language"
)

var Default *Localizer

func init() {
	// init default localizer.
	Default = NewLocalizer(i18n.DefaultLang)
}

// SetLang sets the preferred language
func SetLang(lang string) error {
	langTag, err := language.Parse(lang)
	if err != nil {
		return err
	}
	if i18n.Default.Get(langTag) == nil {
		return fmt.Errorf("language %q not supported", lang)
	}
	Default = NewLocalizer(langTag)
	return nil
}

type Localizer struct {
	lang language.Tag
}

// NewLocalizer creates a new Localizer that looks up messages in the bundle
// according to the language preferences.
func NewLocalizer(lang language.Tag) *Localizer {
	return &Localizer{lang: lang}
}

func (l Localizer) RenderEcode(ecode string, data any) *i18n.EcodeDetail {
	return i18n.Default.RenderEcode(l.lang, ecode, data)
}

func (l *Localizer) RenderMessage(key string, data any) string {
	return i18n.Default.RenderMessage(l.lang, key, data)
}
