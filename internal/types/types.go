package types

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/encoding/prototext"
)

// refer: https://github.com/google/re2/wiki/Syntax
const nameCharSet = `0-9A-Za-z_`
const typeCharSet = `0-9A-Za-z _,\.<>`
const nestedTypeCharSet = typeCharSet + `<>\[\]\{\}`

const nameCharClass = `[` + nameCharSet + `]`
const typeCharClass = `[` + typeCharSet + `]`
const nestedTypeCharClass = `[` + nestedTypeCharSet + `]`

const rawPropGroup = `( *\| *\{(?P<Prop>.+)\})?`

// ungreedy type group
const TypeGroup = `(` + nestedTypeCharClass + `*?)`

// Map definition patterns:
//   - map<KeyType, ValueType>
//   - map<KeyType, .PredefinedValueType>
//   - map<.PredefinedKeyType, ValueType>
//   - map<.PredefinedKeyType, .PredefinedValueType>
var mapRegexp = regexp.MustCompile(`^map<` + `(?P<KeyType>` + nestedTypeCharClass + `+)` + `,` + `(?P<ValueType>` + nestedTypeCharClass + `+)` + `>` + rawPropGroup)

// List definition patterns:
//   - [ElemType]
//   - [ElemType]ColumnType
//   - [.PredefinedElemType]ColumnType
var listRegexp = regexp.MustCompile(`^\[` + `(?P<ElemType>` + nestedTypeCharClass + `*?|@)` + `\]` + `(?P<ColumnType>` + nestedTypeCharClass + `+)?` + rawPropGroup)

// Keyed list definition patterns:
//   - [ElemType]<ColumnType>
//   - [.PredefinedElemType]<ColumnType>
var keyedListRegexp = regexp.MustCompile(`^\[` + `(?P<ElemType>` + nestedTypeCharClass + `*?)` + `\]<` + `(?P<ColumnType>` + nestedTypeCharClass + `+)` + `>` + rawPropGroup)

// Struct definition patterns:
//   - {StructType}
//   - {StructType}ColumnType
//   - {StructType(CustomName)}ColumnType
//   - {.PredefinedStructType}ColumnType
//   - {.PredefinedStructType(CustomName)}ColumnType
const structTypeGroup = `(?P<StructType>` + typeCharClass + `*?)` + `(\((?P<CustomName>` + nameCharClass + `*?)\))?`

var structRegexp = regexp.MustCompile(`^\{` + structTypeGroup + `\}` + `(?P<ColumnType>` + nestedTypeCharClass + `+)?` + rawPropGroup)

// Scalar definition patterns:
//   - int32
//   - string
var scalarRegexp = regexp.MustCompile(`^` + `(?P<ScalarType>` + typeCharClass + `+)` + rawPropGroup)

// Enum definition patterns:
//   - enum<Type>
//   - enum<.PredefinedType>
var enumRegexp = regexp.MustCompile(`^enum<` + `(?P<EnumType>` + typeCharClass + `+)` + `>` + rawPropGroup)

// Field property definition patterns:
//   - |{range:"1,10" refer:"XXXConf.ID"}
//   - | {range:"1,10" refer:"XXXConf.ID"}
var propRegexp = regexp.MustCompile(rawPropGroup)

// trim float to integer after(include) dot, e.g: 0.0, 1.0, 1.00 ...
// refer: https://stackoverflow.com/questions/638565/parsing-scientific-notation-sensibly
var boringIntegerRegexp = regexp.MustCompile(`([-+]?[0-9]+)\.0+$`)

type MapDescriptor struct {
	KeyType   string
	ValueType string
	Prop      PropDescriptor
}

// MatchMap matches the map type patterns. For example:
//   - map<KeyType, ValueType>
//   - map<KeyType, .PredefinedValueType>
//   - map<.PredefinedKeyType, ValueType>
//   - map<.PredefinedKeyType, .PredefinedValueType>
func MatchMap(text string) *MapDescriptor {
	match := mapRegexp.FindStringSubmatch(text)
	if match == nil {
		return nil
	}
	desc := &MapDescriptor{}
	for i, name := range mapRegexp.SubexpNames() {
		value := strings.TrimSpace(match[i])
		switch name {
		case "KeyType":
			desc.KeyType = value
		case "ValueType":
			desc.ValueType = value
		case "Prop":
			desc.Prop.Text = value
		}
	}
	return desc
}

// IsMap checks if text matches the map type patterns.
func IsMap(text string) bool {
	return MatchMap(text) != nil
}

type ListDescriptor struct {
	ElemType   string
	ColumnType string
	Prop       PropDescriptor
}

// MatchList matches the list type patterns. For example:
//   - [ElemType]
//   - [ElemType]Type
//   - [.PredefinedElemType]Type
func MatchList(text string) *ListDescriptor {
	match := listRegexp.FindStringSubmatch(text)
	if match == nil {
		return nil
	}
	desc := &ListDescriptor{}
	for i, name := range listRegexp.SubexpNames() {
		value := strings.TrimSpace(match[i])
		switch name {
		case "ElemType":
			desc.ElemType = value
		case "ColumnType":
			desc.ColumnType = value
		case "Prop":
			desc.Prop.Text = value
		}
	}
	return desc
}

// IsList checks if text matches the list type patterns.
func IsList(text string) bool {
	return MatchList(text) != nil
}

type KeyedListDescriptor struct {
	ElemType   string
	ColumnType string
	Prop       PropDescriptor
}

// MatchKeyedList matches the keyed list type patterns. For example:
//   - [ElemType]<Type>
//   - [.PredefinedElemType]<Type>
func MatchKeyedList(text string) *KeyedListDescriptor {
	match := keyedListRegexp.FindStringSubmatch(text)
	if match == nil {
		return nil
	}
	desc := &KeyedListDescriptor{}
	for i, name := range keyedListRegexp.SubexpNames() {
		value := strings.TrimSpace(match[i])
		switch name {
		case "ElemType":
			desc.ElemType = value
		case "ColumnType":
			desc.ColumnType = value
		case "Prop":
			desc.Prop.Text = value
		}
	}
	return desc
}

// IsKeyedList checks if text matches the keyed list type patterns.
func IsKeyedList(text string) bool {
	return MatchKeyedList(text) != nil
}

type StructDescriptor struct {
	StructType string
	CustomName string
	ColumnType string
	Prop       PropDescriptor
}

// MatchStruct matches the struct type patterns. For example:
//   - {StructType}Type
//   - {StructType(CustomName)}Type
//   - {.PredefinedStructType}Type
//   - {.PredefinedStructType(CustomName)}Type
func MatchStruct(text string) *StructDescriptor {
	match := structRegexp.FindStringSubmatch(text)
	if match == nil {
		return nil
	}
	desc := &StructDescriptor{}
	for i, name := range structRegexp.SubexpNames() {
		value := strings.TrimSpace(match[i])
		switch name {
		case "StructType":
			desc.StructType = value
		case "CustomName":
			desc.CustomName = value
		case "ColumnType":
			desc.ColumnType = value
		case "Prop":
			desc.Prop.Text = value
		}
	}
	return desc
}

// IsStruct checks if text matches the struct type patterns.
func IsStruct(text string) bool {
	return MatchStruct(text) != nil
}

type ScalarDescriptor struct {
	ScalarType string
	Prop       PropDescriptor
}

// MatchScalar matches the scalar type pattern. For example:
//   - int32
//   - string
func MatchScalar(text string) *ScalarDescriptor {
	match := scalarRegexp.FindStringSubmatch(text)
	if match == nil {
		return nil
	}
	desc := &ScalarDescriptor{}
	for i, name := range scalarRegexp.SubexpNames() {
		value := strings.TrimSpace(match[i])
		switch name {
		case "ScalarType":
			desc.ScalarType = value
		case "Prop":
			desc.Prop.Text = value
		}
	}
	return desc
}

type EnumDescriptor struct {
	EnumType string
	Prop     PropDescriptor
}

// MatchEnum matches the enum type pattern. For example:
//   - enum<Type>
//   - enum<.PredefinedType>
func MatchEnum(text string) *EnumDescriptor {
	match := enumRegexp.FindStringSubmatch(text)
	if match == nil {
		return nil
	}
	desc := &EnumDescriptor{}
	for i, name := range enumRegexp.SubexpNames() {
		value := strings.TrimSpace(match[i])
		switch name {
		case "EnumType":
			desc.EnumType = value
		case "Prop":
			desc.Prop.Text = value
		}
	}
	return desc
}

// IsEnum checks if text matches the enum type patterns.
func IsEnum(text string) bool {
	return MatchEnum(text) != nil
}

type PropDescriptor struct {
	Text string // serialized prototext of tableaupb.FieldProp
}

func (x *PropDescriptor) RawProp() string {
	return "|{" + x.Text + "}"
}

func (x *PropDescriptor) FieldProp() (*tableaupb.FieldProp, error) {
	if x.Text == "" {
		return nil, nil
	}
	fieldProp := &tableaupb.FieldProp{}
	if err := prototext.Unmarshal([]byte(x.Text), fieldProp); err != nil {
		return nil, xerrors.ErrorKV(fmt.Sprintf("failed to parse field prop: %s", err), xerrors.KeyPBFieldOpts, x.Text)
	}
	return fieldProp, nil
}

func MatchProp(text string) *PropDescriptor {
	match := propRegexp.FindStringSubmatch(text)
	if match == nil {
		return nil
	}
	prop := &PropDescriptor{}
	for i, name := range propRegexp.SubexpNames() {
		value := strings.TrimSpace(match[i])
		switch name {
		case "Prop":
			prop.Text = value
		}
	}
	return prop
}

func MatchBoringInteger(text string) []string {
	return boringIntegerRegexp.FindStringSubmatch(text)
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

		// well-known message type
		WellKnownMessageTimestamp:  ScalarKind,
		WellKnownMessageDuration:   ScalarKind,
		WellKnownMessageFraction:   ScalarKind,
		WellKnownMessageComparator: ScalarKind,

		// "enum":     EnumKind,
		// "repeated": ListKind,
		// "map":      MapKind,
	}
}

func IsScalarType(fullTypeName string) bool {
	if kind, ok := typeKindMap[fullTypeName]; ok {
		return kind == ScalarKind
	}
	return false
}

// Descriptor describes type metadata.
type Descriptor struct {
	Name       string
	FullName   string
	Predefined bool
	Kind       Kind
}

func ParseTypeDescriptor(rawType string) *Descriptor {
	switch rawType {
	case "datetime", "date":
		return &Descriptor{
			Name:       WellKnownMessageTimestamp,
			FullName:   WellKnownMessageTimestamp,
			Predefined: true,
			Kind:       ScalarKind,
		}
	case "time", "duration":
		return &Descriptor{
			Name:       WellKnownMessageDuration,
			FullName:   WellKnownMessageDuration,
			Predefined: true,
			Kind:       ScalarKind,
		}
	case "fraction":
		return &Descriptor{
			Name:       WellKnownMessageFraction,
			FullName:   WellKnownMessageFraction,
			Predefined: true,
			Kind:       ScalarKind,
		}
	case "comparator":
		return &Descriptor{
			Name:       WellKnownMessageComparator,
			FullName:   WellKnownMessageComparator,
			Predefined: true,
			Kind:       ScalarKind,
		}
	default:
		desc := &Descriptor{
			Name:       rawType,
			FullName:   rawType,
			Predefined: false,
		}
		if IsScalarType(desc.Name) {
			desc.Kind = ScalarKind
		} else if MatchEnum(rawType) != nil {
			desc.Kind = EnumKind
		} else {
			desc.Kind = MessageKind
		}
		return desc
	}
}
