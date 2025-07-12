package fieldprop

import (
	"math"
	"strconv"
	"strings"

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

func RequireSequence(prop *tableaupb.FieldProp) bool {
	return prop != nil && prop.Sequence != nil
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
