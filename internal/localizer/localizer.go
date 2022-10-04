package localizer

import (
	"bytes"
	"embed"
	"fmt"
	"reflect"
	"strings"
	"text/template"

	"gopkg.in/yaml.v2"
)

// TODO: https://go.dev/blog/matchlang

//go:embed i18n
var localeFS embed.FS

// Initialize a slice which holds supported locales.
var languages = []string{"en", "zh"}

var localizers map[string]*Localizer

func Get(lang string) *Localizer {
	return localizers[lang]
}

type Localizer struct {
	lang   string
	ecodes ecodeMap
	tmpl   *template.Template
}

func (l Localizer) RenderEcode(ecode string, data interface{}) *EcodeDetail {
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

func (l Localizer) RenderSummary(module string, data interface{}) string {
	buf := bytes.NewBufferString("")
	name := fmt.Sprintf("%s_%s.tmpl", module, l.lang)
	if err := l.tmpl.ExecuteTemplate(buf, name, data); err != nil {
		panic(err)
	}
	return buf.String()
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

func init() {
	localizers = make((map[string]*Localizer))
	for _, lang := range languages {
		// init ecode
		filename := "i18n/ecode/" + lang + ".yaml"
		data, err := localeFS.ReadFile(filename)
		if err != nil {
			panic(err)
		}
		var ecodes ecodeMap
		if err := yaml.Unmarshal(data, &ecodes); err != nil {
			panic(err)
		}
		// init summary
		// refer: https://stackoverflow.com/questions/22367337/last-item-in-a-template-range
		funcMap := template.FuncMap{
			"last": func(x int, a interface{}) bool {
				return x == reflect.ValueOf(a).Len()-1
			},
			"notlast": func(x int, a interface{}) bool {
				return x != reflect.ValueOf(a).Len()-1
			},
			"hasprefix": strings.HasPrefix,
		}
		t := template.New("ERR-TMPL")
		t, err = t.Funcs(funcMap).ParseFS(localeFS, "i18n/summary/*.tmpl")
		if err != nil {
			panic(err)
		}

		localizers[lang] = &Localizer{
			lang:   lang,
			ecodes: ecodes,
			tmpl:   t,
		}
	}
	// set default localizer of lang
	err := SetLang(lang)
	if err != nil {
		panic(err)
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
