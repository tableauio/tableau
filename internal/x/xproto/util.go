package xproto

import (
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Clone returns a deep copy of m. If the top-level message is invalid, it
// returns an invalid message as well.
func Clone[T proto.Message](m T) T {
	return proto.Clone(m).(T)
}

// GetFieldTypeName parses and returns correct field type name in desired
// format from field descriptor.
//
// The desired formats are:
//   - map<KeyType, ValueType>
//   - repeated ElemType
//   - MessageType
//   - EnumType
//   - ScalarType
func GetFieldTypeName(fd protoreflect.FieldDescriptor) string {
	if fd.IsMap() {
		keyType := fd.MapKey().Kind().String()
		valueType := fd.MapValue().Kind().String()

		if fd.MapValue().Kind() == protoreflect.MessageKind {
			valueType = string(fd.MapValue().Message().FullName())
		} else if fd.MapValue().Kind() == protoreflect.EnumKind {
			valueType = string(fd.MapValue().Enum().FullName())
		}

		return fmt.Sprintf("map<%s, %s>", keyType, valueType)
	} else if fd.IsList() {
		elementType := fd.Kind().String()

		if fd.Kind() == protoreflect.MessageKind {
			elementType = string(fd.Message().FullName())
		} else if fd.Kind() == protoreflect.EnumKind {
			elementType = string(fd.Enum().FullName())
		}

		return fmt.Sprintf("repeated %s", elementType)
	} else if fd.Kind() == protoreflect.MessageKind {
		return string(fd.Message().FullName())
	} else if fd.Kind() == protoreflect.EnumKind {
		return string(fd.Enum().FullName())
	} else {
		return fd.Kind().String()
	}
}
