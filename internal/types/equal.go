package types

import (
	"bytes"
	"math"

	"google.golang.org/protobuf/proto"
	pref "google.golang.org/protobuf/reflect/protoreflect"
)

// EqualMessage reports whether two messages are equal.
func EqualMessage(v1, v2 pref.Value) bool {
	return proto.Equal(v1.Message().Interface(), v2.Message().Interface())
}

// EqualValue compares two singular values.
// NOTE(wenchy): borrowed from https://github.com/protocolbuffers/protobuf-go/blob/v1.27.1/proto/equal.go#L113
func EqualValue(fd pref.FieldDescriptor, v1, v2 pref.Value) bool {
	switch fd.Kind() {
	case pref.BoolKind:
		return v1.Bool() == v2.Bool()
	case pref.EnumKind:
		return v1.Enum() == v2.Enum()
	case pref.Int32Kind, pref.Sint32Kind,
		pref.Int64Kind, pref.Sint64Kind,
		pref.Sfixed32Kind, pref.Sfixed64Kind:
		return v1.Int() == v2.Int()
	case pref.Uint32Kind, pref.Uint64Kind,
		pref.Fixed32Kind, pref.Fixed64Kind:
		return v1.Uint() == v2.Uint()
	case pref.FloatKind, pref.DoubleKind:
		fx := v1.Float()
		fy := v2.Float()
		if math.IsNaN(fx) || math.IsNaN(fy) {
			return math.IsNaN(fx) && math.IsNaN(fy)
		}
		return fx == fy
	case pref.StringKind:
		return v1.String() == v2.String()
	case pref.BytesKind:
		return bytes.Equal(v1.Bytes(), v2.Bytes())
	case pref.MessageKind, pref.GroupKind:
		return EqualMessage(v1, v2)
	default:
		return v1.Interface() == v2.Interface()
	}
}
