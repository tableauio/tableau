package types

import (
	"bytes"
	"math"

	"google.golang.org/protobuf/proto"
	pref "google.golang.org/protobuf/reflect/protoreflect"
)

// equalMessage reports whether two messages are equal.
// If two messages marshal to the same bytes under deterministic serialization,
// then Equal is guaranteed to report true.
func EqualMessage(v1, v2 pref.Value) bool {
	if proto.Equal(v1.Message().Interface(), v2.Message().Interface()) {
		// atom.Log.Debug("empty message exists")
		return true
	}
	return false
}

// equalValue compares two singular values.
// NOTE(wenchy): borrowed from https://github.com/protocolbuffers/protobuf-go/blob/v1.27.1/proto/equal.go#L113
func EqualValue(fd pref.FieldDescriptor, x, y pref.Value) bool {
	switch fd.Kind() {
	case pref.BoolKind:
		return x.Bool() == y.Bool()
	case pref.EnumKind:
		return x.Enum() == y.Enum()
	case pref.Int32Kind, pref.Sint32Kind,
		pref.Int64Kind, pref.Sint64Kind,
		pref.Sfixed32Kind, pref.Sfixed64Kind:
		return x.Int() == y.Int()
	case pref.Uint32Kind, pref.Uint64Kind,
		pref.Fixed32Kind, pref.Fixed64Kind:
		return x.Uint() == y.Uint()
	case pref.FloatKind, pref.DoubleKind:
		fx := x.Float()
		fy := y.Float()
		if math.IsNaN(fx) || math.IsNaN(fy) {
			return math.IsNaN(fx) && math.IsNaN(fy)
		}
		return fx == fy
	case pref.StringKind:
		return x.String() == y.String()
	case pref.BytesKind:
		return bytes.Equal(x.Bytes(), y.Bytes())
	// case pref.MessageKind, pref.GroupKind:
	// 	return equalMessage(x.Message(), y.Message())
	default:
		return x.Interface() == y.Interface()
	}
}