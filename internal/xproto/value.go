package xproto

import (
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
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

func getFieldDefaultValue(fd pref.FieldDescriptor) string {
	opts := fd.Options().(*descriptorpb.FieldOptions)
	fieldOpts := proto.GetExtension(opts, tableaupb.E_Field).(*tableaupb.FieldOptions)
	if fieldOpts != nil && fieldOpts.Prop != nil {
		return fieldOpts.Prop.Default
	}
	return ""
}

func ParseFieldValue(fd pref.FieldDescriptor, rawValue string, locationName string) (v pref.Value, present bool, err error) {
	purifyInteger := func(s string) string {
		// trim integer boring suffix matched by regexp `.0*$`
		if matches := types.MatchBoringInteger(s); matches != nil {
			return matches[1]
		}
		return s
	}

	value := strings.TrimSpace(rawValue)
	defaultValue := getFieldDefaultValue(fd)
	if value == "" {
		value = strings.TrimSpace(defaultValue)
	}

	switch fd.Kind() {
	case pref.Int32Kind, pref.Sint32Kind, pref.Sfixed32Kind:
		if value == "" {
			return DefaultInt32Value, false, nil
		}
		// val, err := strconv.ParseInt(value, 10, 32)
		// Keep compatibility with excel number format.
		// maybe:
		// - decimal fraction: 1.0
		// - scientific notation: 1.0000001e7
		val, err := strconv.ParseFloat(value, 64)
		return pref.ValueOf(int32(val)), true, errors.WithStack(err)

	case pref.Uint32Kind, pref.Fixed32Kind:
		if value == "" {
			return DefaultUint32Value, false, nil
		}
		// val, err := strconv.ParseUint(value, 10, 32)
		// Keep compatibility with excel number format.
		val, err := strconv.ParseFloat(value, 64)
		return pref.ValueOf(uint32(val)), true, errors.WithStack(err)
	case pref.Int64Kind, pref.Sint64Kind, pref.Sfixed64Kind:
		if value == "" {
			return DefaultInt64Value, false, nil
		}
		// val, err := strconv.ParseInt(value, 10, 64)
		// Keep compatibility with excel number format.
		val, err := strconv.ParseFloat(value, 64)
		return pref.ValueOf(int64(val)), true, errors.WithStack(err)
	case pref.Uint64Kind, pref.Fixed64Kind:
		if value == "" {
			return DefaultUint64Value, false, nil
		}
		// val, err := strconv.ParseUint(value, 10, 64)
		// Keep compatibility with excel number format.
		val, err := strconv.ParseFloat(value, 64)
		return pref.ValueOf(uint64(val)), true, errors.WithStack(err)
	case pref.BoolKind:
		if value == "" {
			return DefaultBoolValue, false, nil
		}
		// Keep compatibility with excel number format.
		val, err := strconv.ParseBool(purifyInteger(value))
		return pref.ValueOf(val), true, errors.WithStack(err)

	case pref.FloatKind:
		if value == "" {
			return DefaultFloat32Value, false, nil
		}
		val, err := strconv.ParseFloat(value, 32)
		return pref.ValueOf(float32(val)), true, errors.WithStack(err)

	case pref.DoubleKind:
		if value == "" {
			return DefaultFloat64Value, false, nil
		}
		val, err := strconv.ParseFloat(value, 64)
		return pref.ValueOf(float64(val)), true, errors.WithStack(err)

	case pref.StringKind:
		val := rawValue
		if rawValue == "" {
			val = defaultValue
		}
		return pref.ValueOf(val), val != "", nil
	case pref.BytesKind:
		val := rawValue
		if rawValue == "" {
			val = defaultValue
		}
		return pref.ValueOf([]byte(val)), val != "", nil
	case pref.EnumKind:
		return parseEnumValue(fd, value)
	case pref.MessageKind:
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
			return pref.ValueOf(ts.ProtoReflect()), true, nil

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
			return pref.ValueOf(du.ProtoReflect()), true, nil

		default:
			return pref.Value{}, false, errors.Errorf("not supported message type: %s", msgName)
		}
	// case pref.GroupKind:
	// 	atom.Log.Panicf("not supported key type: %s", fd.Kind().String())
	// 	return pref.Value{}
	default:
		return pref.Value{}, false, errors.Errorf("not supported scalar type: %s", fd.Kind().String())
	}
}

func parseEnumValue(fd pref.FieldDescriptor, rawValue string) (v pref.Value, present bool, err error) {
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
		evd := ed.Values().ByNumber(pref.EnumNumber(int32(val)))
		if evd != nil {
			return pref.ValueOfEnum(evd.Number()), true, nil
		}
		return DefaultEnumValue, true, errors.Errorf("enum: enum value name not defined: %v", value)
	}

	// try enum value name
	evd := ed.Values().ByName(pref.Name(value))
	if evd != nil {
		return pref.ValueOfEnum(evd.Number()), true, nil
	}
	// try enum value alias name
	for i := 0; i < ed.Values().Len(); i++ {
		// get enum value descriptor
		evd := ed.Values().Get(i)
		opts := evd.Options().(*descriptorpb.EnumValueOptions)
		evalueOpts := proto.GetExtension(opts, tableaupb.E_Evalue).(*tableaupb.EnumValueOptions)
		if evalueOpts != nil && evalueOpts.Name == value {
			// alias name found and return
			return pref.ValueOfEnum(evd.Number()), true, nil
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
