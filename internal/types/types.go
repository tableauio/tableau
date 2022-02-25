package types

import (
	"regexp"
	"strings"

	"github.com/tableauio/tableau/internal/atom"
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

const rawpropRegex = `(\|\{.+\})?` // e.g.: |{range:"1,10" refer:"XXXConf.ID"}
const listFirstFieldType = `([0-9A-Za-z_><]+)`

func init() {
	mapRegexp = regexp.MustCompile(`^map<(.+),(.+)>` + rawpropRegex)                 // e.g.: map<uint32,Type>
	listRegexp = regexp.MustCompile(`^\[(.*)\]` + listFirstFieldType + rawpropRegex) // e.g.: [Type]uint32
	keyedListRegexp = regexp.MustCompile(`^\[(.*)\]<(.+)>` + rawpropRegex)           // e.g.: [Type]<uint32>
	structRegexp = regexp.MustCompile(`^\{(.+)\}(.+)` + rawpropRegex)                // e.g.: {Type}uint32
	enumRegexp = regexp.MustCompile(`^enum<(.+)>` + rawpropRegex)                    // e.g.: enum<Type>
	propRegexp = regexp.MustCompile(`\|?\{(.+)\}`)                                   // e.g.: |{range:"1,10" refer:"XXXConf.ID"}

	// trim float to integer after(include) dot, e.g: 0.0, 1.0, 1.00 ...
	// refer: https://stackoverflow.com/questions/638565/parsing-scientific-notation-sensibly
	boringIntegerRegexp = regexp.MustCompile(`([-+]?[0-9]+)\.0+$`)
}

func MatchMap(text string) []string {
	return mapRegexp.FindStringSubmatch(text)
}

func MatchList(text string) []string {
	return listRegexp.FindStringSubmatch(text)
}

func MatchKeyedList(text string) []string {
	return keyedListRegexp.FindStringSubmatch(text)
}

func MatchStruct(text string) []string {
	return structRegexp.FindStringSubmatch(text)
}

func MatchEnum(text string) []string {
	return enumRegexp.FindStringSubmatch(text)
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
			atom.Log.Errorf("parse prop failed: %s", err)
			return nil
		}
		return prop
	}
	return nil
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
