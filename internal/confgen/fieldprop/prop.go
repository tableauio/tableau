package fieldprop

import (
	"math"
	"strconv"
	"strings"

	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// HasUnique checks whether the unique field is set explicitly.
func HasUnique(prop *tableaupb.FieldProp) bool {
	return prop != nil && prop.Unique != nil
}

func RequireUnique(prop *tableaupb.FieldProp) bool {
	return prop != nil && prop.Unique != nil && prop.GetUnique()
}

func CheckInRange(prop *tableaupb.FieldProp, fd protoreflect.FieldDescriptor, value protoreflect.Value, present bool) error {
	if prop == nil || strings.TrimSpace(prop.Range) == "" {
		return nil
	}
	// not present, and presence not required
	if !present && !prop.Present {
		return nil
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
				return xerrors.Errorf("invalid range left: %s", prop.Range)
			}
			if v < left {
				return xerrors.E2004(v, prop.Range)
			}
		}
		if rightStr != "~" {
			right, err := strconv.ParseInt(rightStr, 10, 64)
			if err != nil {
				return xerrors.Errorf("invalid range right: %s", prop.Range)
			}
			if v > right {
				return xerrors.E2004(v, prop.Range)
			}
		}
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		v := value.Uint()
		if leftStr != "~" {
			left, err := strconv.ParseUint(leftStr, 10, 64)
			if err != nil {
				return xerrors.Errorf("invalid range(left): %s", prop.Range)
			}
			if v < left {
				return xerrors.E2004(v, prop.Range)
			}
		}
		if rightStr != "~" {
			right, err := strconv.ParseUint(rightStr, 10, 64)
			if err != nil {
				return xerrors.Errorf("invalid range right: %s", prop.Range)
			}
			if v > right {
				return xerrors.E2004(v, prop.Range)
			}
		}
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		v := value.Float()
		if leftStr != "~" {
			left, err := strconv.ParseFloat(leftStr, 64)
			if err != nil {
				return xerrors.Errorf("invalid range left: %s", prop.Range)
			}
			if v < left {
				return xerrors.E2004(v, prop.Range)
			}
		}
		if rightStr != "~" {
			right, err := strconv.ParseFloat(rightStr, 64)
			if err != nil {
				return xerrors.Errorf("invalid range right: %s", prop.Range)
			}
			if v > right {
				return xerrors.E2004(v, prop.Range)
			}
		}
	}
	return nil
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
		return 0, xerrors.Errorf("not supported sequence kind: %s", kind)
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
		return prevValue, xerrors.Errorf("not supported sequence kind: %s", kind)
	}
	return prevValue, nil
}

// IsFixed check the horizontal list/map is fixed size or not.
func IsFixed(prop *tableaupb.FieldProp) bool {
	if prop != nil {
		return prop.Fixed || prop.Size > 0
	}
	return false
}

// GetSize returns the specified size of horizontal list/map.
// detectedSize is the scanned size of name row.
func GetSize(prop *tableaupb.FieldProp, detectedSize int) int {
	if prop != nil {
		if prop.Size > 0 {
			return int(prop.Size)
		} else if prop.Fixed {
			return detectedSize
		}
	}
	return 0
}

func CheckPresence(prop *tableaupb.FieldProp, present bool) error {
	if prop != nil && prop.Present {
		if !present {
			return xerrors.E2011()
		}
	}
	return nil
}

// GetUnionCrossFieldCount returns the cross field count of a union
// list (TODO: map) field. If cross is < 0, means occupying all following
// fields, so it will return [math.MaxInt].
func GetUnionCrossFieldCount(prop *tableaupb.FieldProp) int {
	if prop != nil {
		if prop.Cross < 0 {
			return math.MaxInt
		}
		return int(prop.Cross)
	}
	return 0
}
