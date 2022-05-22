package xproto

import (
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	pref "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var DefaultBoolValue pref.Value
var DefaultInt32Value pref.Value
var DefaultUint32Value pref.Value
var DefaultInt64Value pref.Value
var DefaultUint64Value pref.Value
var DefaultFloat32Value pref.Value
var DefaultFloat64Value pref.Value
var DefaultStringValue pref.Value
var DefaultBytesValue pref.Value
var DefaultEnumValue pref.Value

var DefaultTimestampValue pref.Value
var DefaultDurationValue pref.Value

func init() {
	DefaultBoolValue = pref.ValueOf(false)
	DefaultInt32Value = pref.ValueOf(int32(0))
	DefaultUint32Value = pref.ValueOf(uint32(0))
	DefaultInt64Value = pref.ValueOf(int64(0))
	DefaultUint64Value = pref.ValueOf(uint64(0))
	DefaultFloat32Value = pref.ValueOf(float32(0))
	DefaultFloat64Value = pref.ValueOf(float64(0))
	DefaultStringValue = pref.ValueOf("")
	DefaultBytesValue = pref.ValueOf([]byte{})
	DefaultEnumValue = pref.ValueOfEnum(pref.EnumNumber(0))

	var ts *timestamppb.Timestamp
	DefaultTimestampValue = pref.ValueOf(ts.ProtoReflect())

	var du *durationpb.Duration
	DefaultDurationValue = pref.ValueOf(du.ProtoReflect())
}

func ParseFieldValue(fd protoreflect.FieldDescriptor, rawValue string, locationName string) (v protoreflect.Value, present bool, err error) {
	purifyInteger := func(s string) string {
		// trim integer boring suffix matched by regexp `.0*$`
		if matches := types.MatchBoringInteger(s); matches != nil {
			return matches[1]
		}
		return s
	}

	value := strings.TrimSpace(rawValue)
	switch fd.Kind() {
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		if value == "" {
			return DefaultInt32Value, false, nil
		}
		// val, err := strconv.ParseInt(value, 10, 32)
		// Keep compatibility with excel number format.
		// maybe:
		// - decimal fraction: 1.0
		// - scientific notation: 1.0000001e7
		val, err := strconv.ParseFloat(value, 64)
		return protoreflect.ValueOf(int32(val)), true, errors.WithStack(err)

	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		if value == "" {
			return DefaultUint32Value, false, nil
		}
		// val, err := strconv.ParseUint(value, 10, 32)
		// Keep compatibility with excel number format.
		val, err := strconv.ParseFloat(value, 64)
		return protoreflect.ValueOf(uint32(val)), true, errors.WithStack(err)
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		if value == "" {
			return DefaultInt64Value, false, nil
		}
		// val, err := strconv.ParseInt(value, 10, 64)
		// Keep compatibility with excel number format.
		val, err := strconv.ParseFloat(value, 64)
		return protoreflect.ValueOf(int64(val)), true, errors.WithStack(err)
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		if value == "" {
			return DefaultUint64Value, false, nil
		}
		// val, err := strconv.ParseUint(value, 10, 64)
		// Keep compatibility with excel number format.
		val, err := strconv.ParseFloat(value, 64)
		return protoreflect.ValueOf(uint64(val)), true, errors.WithStack(err)
	case protoreflect.BoolKind:
		if value == "" {
			return DefaultBoolValue, false, nil
		}
		// Keep compatibility with excel number format.
		val, err := strconv.ParseBool(purifyInteger(value))
		return protoreflect.ValueOf(val), true, errors.WithStack(err)

	case protoreflect.FloatKind:
		if value == "" {
			return DefaultFloat32Value, false, nil
		}
		val, err := strconv.ParseFloat(value, 32)
		return protoreflect.ValueOf(float32(val)), true, errors.WithStack(err)

	case protoreflect.DoubleKind:
		if value == "" {
			return DefaultFloat64Value, false, nil
		}
		val, err := strconv.ParseFloat(value, 64)
		return protoreflect.ValueOf(float64(val)), true, errors.WithStack(err)

	case protoreflect.StringKind:
		return protoreflect.ValueOf(rawValue), rawValue != "", nil
	case protoreflect.BytesKind:
		return protoreflect.ValueOf([]byte(rawValue)), rawValue != "", nil
	case protoreflect.EnumKind:
		return parseEnumValue(fd, value)
	case protoreflect.MessageKind:
		msgName := fd.Message().FullName()
		switch msgName {
		case "google.protobuf.Timestamp":
			if value == "" {
				return DefaultTimestampValue, false, nil
			}
			// location name examples: "Asia/Shanghai" or "Asia/Chongqing".
			// NOTE(wenchy): There is no "Asia/Beijing" location name. Whoa!!! Big surprize?
			t, err := parseTimeWithLocation(locationName, value)
			if err != nil {
				return DefaultTimestampValue, true, errors.WithMessagef(err, "illegal timestamp string format: %v", value)
			}
			// atom.Log.Debugf("timeStr: %v, unix timestamp: %v", value, t.Unix())
			ts := timestamppb.New(t)
			if err := ts.CheckValid(); err != nil {
				return DefaultTimestampValue, true, errors.WithMessagef(err, "invalid timestamp: %v", value)
			}
			return protoreflect.ValueOf(ts.ProtoReflect()), true, nil

		case "google.protobuf.Duration":
			if value == "" {
				return DefaultDurationValue, false, nil
			}
			d, err := parseDuration(value)
			if err != nil {
				return DefaultDurationValue, true, errors.WithMessagef(err, "illegal duration string format: %v", value)
			}
			du := durationpb.New(d)
			if err := du.CheckValid(); err != nil {
				return DefaultDurationValue, true, errors.WithMessagef(err, "invalid duration: %v", value)
			}
			return protoreflect.ValueOf(du.ProtoReflect()), true, nil

		default:
			return protoreflect.Value{}, false, errors.Errorf("not supported message type: %s", msgName)
		}
	// case protoreflect.GroupKind:
	// 	atom.Log.Panicf("not supported key type: %s", fd.Kind().String())
	// 	return protoreflect.Value{}
	default:
		return protoreflect.Value{}, false, errors.Errorf("not supported scalar type: %s", fd.Kind().String())
	}
}

func parseEnumValue(fd protoreflect.FieldDescriptor, rawValue string) (v protoreflect.Value, present bool, err error) {
	// default enum value
	value := strings.TrimSpace(rawValue)
	if value == "" {
		return DefaultEnumValue, false, nil
	}
	ed := fd.Enum() // get enum descriptor

	// try enum value number
	// val, err := strconv.ParseInt(value, 10, 32)

	// Keep compatibility with excel number format.
	val, err := strconv.ParseFloat(value, 64)
	if err == nil {
		evd := ed.Values().ByNumber(protoreflect.EnumNumber(int32(val)))
		if evd != nil {
			return protoreflect.ValueOfEnum(evd.Number()), true, nil
		}
		return DefaultEnumValue, true, errors.Errorf("enum: enum value name not defined: %v", value)
	}

	// try enum value name
	evd := ed.Values().ByName(protoreflect.Name(value))
	if evd != nil {
		return protoreflect.ValueOfEnum(evd.Number()), true, nil
	}
	// try enum value alias name
	for i := 0; i < ed.Values().Len(); i++ {
		// get enum value descriptor
		evd := ed.Values().Get(i)
		opts := evd.Options().(*descriptorpb.EnumValueOptions)
		evalueOpts := proto.GetExtension(opts, tableaupb.E_Evalue).(*tableaupb.EnumValueOptions)
		if evalueOpts != nil && evalueOpts.Name == value {
			// alias name found and return
			return protoreflect.ValueOfEnum(evd.Number()), true, nil
		}
	}
	return DefaultEnumValue, true, errors.Errorf("enum: enum(%s) value options not found: %v", ed.FullName(), value)
}

func parseTimeWithLocation(locationName string, timeStr string) (time.Time, error) {
	// see https://golang.org/pkg/time/#LoadLocation
	if location, err := time.LoadLocation(locationName); err != nil {
		return time.Time{}, errors.Wrapf(err, "LoadLocation failed: %s", locationName)
	} else {
		timeStr = strings.TrimSpace(timeStr)
		layout := "2006-01-02 15:04:05"
		if strings.Contains(timeStr, " ") {
			layout = "2006-01-02 15:04:05"
		} else {
			layout = "2006-01-02"
			if !strings.Contains(timeStr, "-") && len(timeStr) == 8 {
				// convert "yyyymmdd" to "yyyy-mm-dd"
				timeStr = timeStr[0:4] + "-" + timeStr[4:6] + "-" + timeStr[6:8]
			}
		}
		t, err := time.ParseInLocation(layout, timeStr, location)
		if err != nil {
			return time.Time{}, errors.Wrapf(err, "ParseInLocation failed, timeStr: %v, locationName: %v", timeStr, locationName)
		}
		return t, nil
	}
}

func parseDuration(duration string) (time.Duration, error) {
	duration = strings.TrimSpace(duration)
	if !strings.ContainsAny(duration, ":hmsµu") && len(duration) == 6 {
		duration = duration[0:2] + "h" + duration[2:4] + "m" + duration[4:6] + "s"
	} else if strings.Contains(duration, ":") && len(duration) == 8 {
		// convert "hh:mm:ss" to "<hh>h<mm>m:<ss>s"
		duration = duration[0:2] + "h" + duration[3:5] + "m" + duration[6:8] + "s"
	}

	return time.ParseDuration(duration)
}
