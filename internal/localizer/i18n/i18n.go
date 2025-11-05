package i18n

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

// TODO: learn more about Internationalization (i18n) and Localization (l10n)
// - https://go.dev/blog/matchlang
// - https://www.alexedwards.net/blog/i18n-managing-translations
// - https://github.com/nicksnyder/go-i18n

//go:embed config
var localeFS embed.FS

var DefaultLang language.Tag = language.English

// Initialize a slice which holds supported languages.
var languages = []string{"en", "zh"}

type I18N struct {
	bundles map[string]*Bundle // lang -> *Bundle
}

func (i *I18N) Get(lang language.Tag) *Bundle {
	return i.bundles[lang.String()]
}

func (i *I18N) GetDefault() *Bundle {
	return i.bundles[DefaultLang.String()]
}

func (i *I18N) RenderEcode(lang language.Tag, ecode string, data any) *EcodeDetail {
	bundle := i.Get(lang)
	if bundle == nil {
		panic(fmt.Sprintf("language %q not supported", lang))
	}
	detail, err := bundle.RenderEcode(ecode, data)
	if err != nil {
		// fallback to default language
		detail, err = i.GetDefault().RenderEcode(ecode, data)
		if err != nil {
			panic(err)
		}
		return detail
	}
	return detail
}

func (i I18N) RenderMessage(lang language.Tag, key string, data any) string {
	bundle := i.Get(lang)
	if bundle == nil {
		panic(fmt.Sprintf("language %q not supported", lang))
	}
	text, err := bundle.RenderMessage(key, data)
	if err != nil {
		// fallback to default language
		text, err = i.GetDefault().RenderMessage(key, data)
		if err != nil {
			panic(err)
		}
	}
	return render(text, data)
}

type Bundle struct {
	lang string
	// ecode -> ecode detail
	ecodes map[string]EcodeDetail
	// ID -> message
	messages map[string]string
}

func (b Bundle) RenderEcode(ecode string, data any) (*EcodeDetail, error) {
	rawDetail, ok := b.ecodes[ecode]
	if !ok {
		return nil, fmt.Errorf("render ecode: ecode %s not found", ecode)
	}
	return &EcodeDetail{
		Desc: rawDetail.Desc,
		Text: render(rawDetail.Text, data),
		Help: render(rawDetail.Help, data),
	}, nil
}

func (b Bundle) RenderMessage(key string, data any) (string, error) {
	text, ok := b.messages[key]
	if !ok {
		return "", fmt.Errorf("render message: key %s not found", key)
	}
	return render(text, data), nil
}

// See https://rustc-dev-guide.rust-lang.org/diagnostics.html
type EcodeDetail struct {
	Desc   string       `yaml:"desc"`
	Text   string       `yaml:"text"`
	Help   string       `yaml:"help"`
	Fields []EcodeField `yaml:"fields"`
}

// EcodeField maps field name -> field type
type EcodeField map[string]string

func (f EcodeField) Validate() bool {
	return len(f) == 1 && f.Name() != "" && f.Type() != ""
}

// Name returns the field name.
func (f EcodeField) Name() string {
	for k := range f {
		return k
	}
	return ""
}

// Type returns the field type.
func (f EcodeField) Type() string {
	for _, v := range f {
		return v
	}
	return ""
}

var Default *I18N

func init() {
	Default = &I18N{bundles: map[string]*Bundle{}}
	for _, lang := range languages {
		// init ecode
		filename := "config/ecode/" + lang + ".yaml"
		data, err := localeFS.ReadFile(filename)
		if err != nil {
			panic(err)
		}
		var ecodes map[string]EcodeDetail
		if err := yaml.Unmarshal(data, &ecodes); err != nil {
			panic(err)
		}

		// init message
		filename = "config/message/" + lang + ".yaml"
		data, err = localeFS.ReadFile(filename)
		if err != nil {
			panic(err)
		}
		var messages map[string]string
		if err := yaml.Unmarshal(data, &messages); err != nil {
			panic(err)
		}

		Default.bundles[lang] = &Bundle{
			lang:     lang,
			ecodes:   ecodes,
			messages: messages,
		}
	}
}

func render(text string, data any) string {
	tmpl, err := template.New("i18n").Parse(text)
	if err != nil {
		panic(err)
	}
	buf := bytes.NewBufferString("")
	err = tmpl.Execute(buf, data)
	if err != nil {
		panic(err)
	}
	return buf.String()
}
