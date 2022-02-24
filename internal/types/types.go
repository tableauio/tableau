package types

import "regexp"

var mapRegexp *regexp.Regexp
var listRegexp *regexp.Regexp
var keyedListRegexp *regexp.Regexp
var structRegexp *regexp.Regexp
var enumRegexp *regexp.Regexp

var boringIntegerRegexp *regexp.Regexp

func init() {
	mapRegexp = regexp.MustCompile(`^map<(.+),(.+)>`)       // e.g.: map<uint32,Type>|{range:"1,10" refer:"XXXConf.ID"}
	listRegexp = regexp.MustCompile(`^\[(.*)\](.+)`)        // e.g.: [Type]uint32
	keyedListRegexp = regexp.MustCompile(`^\[(.*)\]<(.+)>`) // e.g.: [Type]<uint32>
	structRegexp = regexp.MustCompile(`^\{(.+)\}(.+)`)      // e.g.: {Type}uint32
	enumRegexp = regexp.MustCompile(`^enum<(.+)>`)          // e.g.: enum<Type>

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

func MatchBoringInteger(text string) []string {
	return boringIntegerRegexp.FindStringSubmatch(text)
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
