package types

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/encoding/prototext"
)

var mapRegexp *regexp.Regexp
var listRegexp *regexp.Regexp
var keyedListRegexp *regexp.Regexp
var structRegexp *regexp.Regexp
var enumRegexp *regexp.Regexp
var propRegexp *regexp.Regexp

var boringIntegerRegexp *regexp.Regexp

// refer: https://github.com/google/re2/wiki/Syntax
const rawPropGroup = `(\|\{.+\})?` // e.g.: |{range:"1,10" refer:"XXXConf.ID"}
const typeCharSet = `[0-9A-Za-z,_>< \[\]\.\{\}]`
const typeGroup = `(` + typeCharSet + `+)`
const looseTypeGroup = typeGroup + `?` // `x?`: zero or one x, prefer one
const ungreedyTypeGroup = `(` + typeCharSet + `*?)`
const TypeGroup = ungreedyTypeGroup

func init() {
	mapRegexp = regexp.MustCompile(`^map<` + typeGroup + `,` + typeGroup + `>` + rawPropGroup)               // e.g.: map<uint32,Type>
	listRegexp = regexp.MustCompile(`^\[` + ungreedyTypeGroup + `\]` + typeGroup + rawPropGroup)             // e.g.: [Type]uint32
	keyedListRegexp = regexp.MustCompile(`^\[` + ungreedyTypeGroup + `\]<` + typeGroup + `>` + rawPropGroup) // e.g.: [Type]<uint32>
	structRegexp = regexp.MustCompile(`^\{` + ungreedyTypeGroup + `\}` + looseTypeGroup + rawPropGroup)      // e.g.: {Type}uint32
	enumRegexp = regexp.MustCompile(`^enum<` + typeGroup + `>` + rawPropGroup)                               // e.g.: enum<Type>
	propRegexp = regexp.MustCompile(`\|?\{(.+)\}`)                                                           // e.g.: |{range:"1,10" refer:"XXXConf.ID"}

	// trim float to integer after(include) dot, e.g: 0.0, 1.0, 1.00 ...
	// refer: https://stackoverflow.com/questions/638565/parsing-scientific-notation-sensibly
	boringIntegerRegexp = regexp.MustCompile(`([-+]?[0-9]+)\.0+$`)
}

func MatchMap(text string) []string {
	return mapRegexp.FindStringSubmatch(text)
}

func IsMap(text string) bool {
	return MatchMap(text) != nil
}

func MatchList(text string) []string {
	return listRegexp.FindStringSubmatch(text)
}

func IsList(text string) bool {
	return MatchList(text) != nil
}

func MatchKeyedList(text string) []string {
	return keyedListRegexp.FindStringSubmatch(text)
}

func IsKeyedList(text string) bool {
	return MatchKeyedList(text) != nil
}

func MatchStruct(text string) []string {
	return structRegexp.FindStringSubmatch(text)
}

func IsStruct(text string) bool {
	return MatchStruct(text) != nil
}

func MatchEnum(text string) []string {
	return enumRegexp.FindStringSubmatch(text)
}

func IsEnum(text string) bool {
	return MatchEnum(text) != nil
}

func MatchProp(text string) []string {
	return propRegexp.FindStringSubmatch(text)
}

func MatchBoringInteger(text string) []string {
	return boringIntegerRegexp.FindStringSubmatch(text)
}

func ParseProp(text string) *tableaupb.FieldProp {
	matches := propRegexp.FindStringSubmatch(text)
	if len(matches) > 0 {
		propText := strings.TrimSpace(matches[1])
		if propText == "" {
			return nil
		}
		prop := &tableaupb.FieldProp{}
		if err := prototext.Unmarshal([]byte(propText), prop); err != nil {
			log.Errorf("parse prop failed: %s", err)
			return nil
		}
		return prop
	}
	return nil
}

// BelongToFirstElement returns true if the name has specified `prefix+"1"`
// and the next character is not digit.
func BelongToFirstElement(name, prefix string) bool {
	firstElemPrefix := prefix + "1"
	nextCharPos := len(firstElemPrefix)
	if strings.HasPrefix(name, firstElemPrefix) {
		if len(name) > len(firstElemPrefix) {
			char := name[nextCharPos]
			return !unicode.IsDigit(rune(char))
		}
	}
	return false
}

type Kind int

const (
	ScalarKind Kind = iota
	EnumKind
	ListKind
	MapKind
	MessageKind
)

var typeKindMap map[string]Kind

func init() {
	typeKindMap = map[string]Kind{
		"bool":     ScalarKind,
		"enum":     ScalarKind,
		"int32":    ScalarKind,
		"sint32":   ScalarKind,
		"uint32":   ScalarKind,
		"int64":    ScalarKind,
		"sint64":   ScalarKind,
		"uint64":   ScalarKind,
		"sfixed32": ScalarKind,
		"fixed32":  ScalarKind,
		"float":    ScalarKind,
		"sfixed64": ScalarKind,
		"fixed64":  ScalarKind,
		"double":   ScalarKind,
		"string":   ScalarKind,
		"bytes":    ScalarKind,

		"repeated": ListKind,
		"map":      MapKind,
	}
}

func IsScalarType(t string) bool {
	if kind, ok := typeKindMap[t]; ok {
		return kind == ScalarKind
	}
	return false
}
