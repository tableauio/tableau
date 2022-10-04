package localizer

import "fmt"

var lang = "en"
var Default *Localizer

func SetLang(language string) error {
	Default = localizers[language]
	if Default == nil {
		return fmt.Errorf("lang %s not found", language)
	}
	lang = language
	return nil
}
