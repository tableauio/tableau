package prop

import (
	"strconv"
	"strings"

	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func IsUnique(prop *tableaupb.FieldProp) bool {
	return prop != nil && prop.Unique
}

func InRange(prop *tableaupb.FieldProp, fd protoreflect.FieldDescriptor, value protoreflect.Value) bool {
	if prop == nil || strings.TrimSpace(prop.Range) == "" {
		return true
	}
	splits := strings.SplitN(prop.Range, ",", 2)
	leftStr := strings.TrimSpace(splits[0])
	rightStr := strings.TrimSpace(splits[1])

	switch fd.Kind() {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		v := value.Int()
		if leftStr != "~" {
			left, err := strconv.ParseInt(leftStr, 10, 64)
			if err != nil {
				atom.Log.Errorf("invalid range left: %s", prop.Range)
				return false
			}
			if v < left {
				return false
			}
		}
		if rightStr != "~" {
			right, err := strconv.ParseInt(rightStr, 10, 64)
			if err != nil {
				atom.Log.Errorf("invalid range right: %s", prop.Range)
				return false
			}
			if v > right {
				return false
			}
		}
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		v := value.Uint()
		if leftStr != "~" {
			left, err := strconv.ParseUint(leftStr, 10, 64)
			if err != nil {
				atom.Log.Errorf("invalid range(left): %s", prop.Range)
				return false
			}
			if v < left {
				return false
			}
		}
		if rightStr != "~" {
			right, err := strconv.ParseUint(rightStr, 10, 64)
			if err != nil {
				atom.Log.Errorf("invalid range right: %s", prop.Range)
				return false
			}
			if v > right {
				return false
			}
		}
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		v := value.Float()
		if leftStr != "~" {
			left, err := strconv.ParseFloat(leftStr, 64)
			if err != nil {
				atom.Log.Errorf("invalid range left: %s", prop.Range)
				return false
			}
			if v < left {
				return false
			}
		}
		if rightStr != "~" {
			right, err := strconv.ParseFloat(rightStr, 64)
			if err != nil {
				atom.Log.Errorf("invalid range right: %s", prop.Range)
				return false
			}
			if v > right {
				return false
			}
		}
	}
	return true
}

func CheckSequence(prop *tableaupb.FieldProp, fd protoreflect.FieldDescriptor, value protoreflect.Value, lastValue protoreflect.Value) bool {
	// if prop == nil || prop.Sequence == nil {
	// 	return true
	// }
	// switch fd.Kind() {
	// case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
	// 	protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
	// 	v := value.Int()
	// case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
	// 	protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
	// 	return false
	// }
	return true
}
