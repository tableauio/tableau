package prop

import (
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/log"
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
				log.Errorf("invalid range left: %s", prop.Range)
				return false
			}
			if v < left {
				return false
			}
		}
		if rightStr != "~" {
			right, err := strconv.ParseInt(rightStr, 10, 64)
			if err != nil {
				log.Errorf("invalid range right: %s", prop.Range)
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
				log.Errorf("invalid range(left): %s", prop.Range)
				return false
			}
			if v < left {
				return false
			}
		}
		if rightStr != "~" {
			right, err := strconv.ParseUint(rightStr, 10, 64)
			if err != nil {
				log.Errorf("invalid range right: %s", prop.Range)
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
				log.Errorf("invalid range left: %s", prop.Range)
				return false
			}
			if v < left {
				return false
			}
		}
		if rightStr != "~" {
			right, err := strconv.ParseFloat(rightStr, 64)
			if err != nil {
				log.Errorf("invalid range right: %s", prop.Range)
				return false
			}
			if v > right {
				return false
			}
		}
	}
	return true
}

func CheckMapKeySequence(prop *tableaupb.FieldProp, kind protoreflect.Kind, mapkey protoreflect.MapKey, prefMap protoreflect.Map) bool {
	if prop == nil || prop.Sequence == nil {
		return true
	}
	if prefMap.Len() == 0 {
		val, err := convertValueToInt64(kind, mapkey.Value())
		if err != nil {
			log.Errorf("convert map key to int64 failed: %s", err)
			return false
		}
		return prop.GetSequence() == val
	}
	prevValue, err := getPrevValueOfSequence(kind, mapkey.Value())
	if err != nil {
		log.Errorf("get prev value of sequence error: %s", err)
		return false
	}
	return prefMap.Has(prevValue.MapKey())
}

func convertValueToInt64(kind protoreflect.Kind, value protoreflect.Value) (int64, error) {
	switch kind {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return value.Int(), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		// NOTE: as sequence type is int64, so convert to int64 even if the value is uint64.
		return int64(value.Uint()), nil
	default:
		return 0, errors.Errorf("not supported sequence kind: %s", kind)
	}
}

func getPrevValueOfSequence(kind protoreflect.Kind, value protoreflect.Value) (protoreflect.Value, error) {
	var prevValue protoreflect.Value
	switch kind {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		prevValue = protoreflect.ValueOfInt32(int32(value.Int()) - 1)
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		prevValue = protoreflect.ValueOfInt64(value.Int() - 1)
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		prevValue = protoreflect.ValueOfUint32(uint32(value.Uint() - 1))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		prevValue = protoreflect.ValueOfUint64(value.Uint() - 1)
	default:
		return prevValue, errors.Errorf("not supported sequence kind: %s", kind)
	}
	return prevValue, nil
}

// IsFixed check the horizontal list/map is fixed length or not.
func IsFixed(prop *tableaupb.FieldProp) bool {
	if prop != nil {
		return prop.Fixed || prop.Length > 0
	}
	return false
}

// GetLength returns the specified length of horizontal list/map.
// existedLength is the scanned length.
func GetLength(prop *tableaupb.FieldProp, existedLength int) int {
	if prop != nil {
		if prop.Length > 0 {
			return int(prop.Length)
		} else if prop.Fixed {
			return existedLength
		}
	}
	return 0
}
