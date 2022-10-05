package i18n

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	"golang.org/x/text/language"
	"gopkg.in/yaml.v2"
)

// TODO: learn more about Internationalization (i18n) and Localization (l10n)
// - https://go.dev/blog/matchlang
// - https://www.alexedwards.net/blog/i18n-managing-translations
// - https://github.com/nicksnyder/go-i18n

//go:embed config
var localeFS embed.FS

// Initialize a slice which holds supported languages.
var languages = []string{"en", "zh"}

var bundles map[string]*Bundle

// NewBundle returns a bundle with a default language and a default set of plural rules.
func NewBundle(defaultLang language.Tag) *Bundle {
	return bundles[defaultLang.String()]
}

type Bundle struct {
	lang     string
	ecodes   ecodeMap
	messages messageMap
}

func (l Bundle) RenderEcode(ecode string, data interface{}) *EcodeDetail {
	rawDetail, ok := l.ecodes[ecode]
	if !ok {
		panic(fmt.Sprintf("ecode %s not found", ecode))
	}
	return &EcodeDetail{
		Ecode: ecode,
		Desc:  rawDetail.Desc,
		Text:  render(rawDetail.Text, data),
		Help:  render(rawDetail.Help, data),
	}
}

func (l Bundle) RenderMessage(key string, data interface{}) string {
	text, ok := l.messages[key]
	if !ok {
		panic(fmt.Sprintf("key %s not found", key))
	}
	return render(text, data)
}

// See https://rustc-dev-guide.rust-lang.org/diagnostics.html
type EcodeDetail struct {
	Ecode string // error code like: EXXXX
	Desc  string `yaml:"desc"` // basic description
	Text  string `yaml:"text"` // error text
	Help  string `yaml:"help"` // fix suggestion
}

// ecode -> ecode detail
type ecodeMap map[string]EcodeDetail
// ID -> message
type messageMap map[string]string

func init() {
	bundles = make((map[string]*Bundle))
	for _, lang := range languages {
		// init ecode
		filename := "config/ecode/" + lang + ".yaml"
		data, err := localeFS.ReadFile(filename)
		if err != nil {
			panic(err)
		}
		var ecodes ecodeMap
		if err := yaml.Unmarshal(data, &ecodes); err != nil {
			panic(err)
		}

		// init message
		filename = "config/message/" + lang + ".yaml"
		data, err = localeFS.ReadFile(filename)
		if err != nil {
			panic(err)
		}
		var messages messageMap
		if err := yaml.Unmarshal(data, &messages); err != nil {
			panic(err)
		}

		bundles[lang] = &Bundle{
			lang:     lang,
			ecodes:   ecodes,
			messages: messages,
		}
	}
}

func render(text string, data interface{}) string {
	tmpl, err := template.New("ERROR").Parse(text)
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