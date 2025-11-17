package xproto

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
	pref "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
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
var DefaultFractionValue pref.Value
var DefaultComparatorValue pref.Value
var DefaultVersionValue pref.Value

func init() {
	DefaultBoolValue = pref.ValueOfBool(false)
	DefaultInt32Value = pref.ValueOfInt32(0)
	DefaultUint32Value = pref.ValueOfUint32(0)
	DefaultInt64Value = pref.ValueOfInt64(0)
	DefaultUint64Value = pref.ValueOfUint64(0)
	DefaultFloat32Value = pref.ValueOfFloat32(0)
	DefaultFloat64Value = pref.ValueOfFloat64(0)
	DefaultStringValue = pref.ValueOfString("")
	DefaultBytesValue = pref.ValueOfBytes(nil)
	DefaultEnumValue = pref.ValueOfEnum(pref.EnumNumber(0))

	var ts *timestamppb.Timestamp
	DefaultTimestampValue = pref.ValueOfMessage(ts.ProtoReflect())

	var du *durationpb.Duration
	DefaultDurationValue = pref.ValueOfMessage(du.ProtoReflect())
}

// ParseFieldValue parses field value by FieldDescriptor. It can parse following
// basic types:
//
// # Scalar types
//
//   - Numbers: int32, uint32, int64, uint64, float, double
//   - Booleans: bool
//   - Strings: string
//   - Bytes: bytes
//
// # Enum types
//
// # Well-known types
//
// Well-known types are message types defined by [types.IsWellKnownMessage].
func ParseFieldValue(fd pref.FieldDescriptor, rawValue string, locationName string, fprop *tableaupb.FieldProp) (v pref.Value, present bool, err error) {
	getTrimmedValue := func() string {
		value := strings.TrimSpace(rawValue)
		if value == "" {
			value = strings.TrimSpace(GetFieldDefaultValue(fd))
		}
		return value
	}

	getValue := func() string {
		value := rawValue
		if value == "" {
			value = GetFieldDefaultValue(fd)
		}
		return value
	}

	switch fd.Kind() {
	case pref.Int32Kind, pref.Sint32Kind, pref.Sfixed32Kind:
		value := getTrimmedValue()
		if value == "" {
			return DefaultInt32Value, false, nil
		}
		// val, err := strconv.ParseInt(value, 10, 32)
		// Keep compatibility with excel number format.
		// maybe:
		// - decimal fraction: 1.0
		// - scientific notation: 1.0000001e7
		//
		// NOTE: IEEE-754 64-bit floats (float64 in Go) can only accurately
		// represent integers in the range [-2^53,2^53].
		val, err := strconv.ParseFloat(value, 64)
		if val < math.MinInt32 || val > math.MaxInt32 {
			return DefaultInt32Value, false, xerrors.E2000("int32", value, math.MinInt32, math.MaxInt32)
		}
		return pref.ValueOfInt32(int32(val)), true, xerrors.E2012("int32", value, err)

	case pref.Uint32Kind, pref.Fixed32Kind:
		value := getTrimmedValue()
		if value == "" {
			return DefaultUint32Value, false, nil
		}
		// val, err := strconv.ParseUint(value, 10, 32)
		// Keep compatibility with excel number format.
		val, err := strconv.ParseFloat(value, 64)
		if val < 0 || val > math.MaxUint32 {
			return DefaultUint32Value, false, xerrors.E2000("uint32", value, 0, math.MaxUint32)
		}
		return pref.ValueOfUint32(uint32(val)), true, xerrors.E2012("uint32", value, err)

	case pref.Int64Kind, pref.Sint64Kind, pref.Sfixed64Kind:
		value := getTrimmedValue()
		if value == "" {
			return DefaultInt64Value, false, nil
		}
		val, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return DefaultInt64Value, false, xerrors.E2012("int64", value, err)
		}
		return pref.ValueOfInt64(val), true, nil

	case pref.Uint64Kind, pref.Fixed64Kind:
		value := getTrimmedValue()
		if value == "" {
			return DefaultUint64Value, false, nil
		}
		val, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return DefaultUint64Value, false, xerrors.E2012("uint64", value, err)
		}
		return pref.ValueOfUint64(val), true, nil

	case pref.BoolKind:
		value := getTrimmedValue()
		if value == "" {
			return DefaultBoolValue, false, nil
		}
		// Keep compatibility with excel number format.
		val, err := ParseBool(value)
		if err != nil {
			return DefaultBoolValue, false, xerrors.E2013(value, err)
		}
		return pref.ValueOfBool(val), true, nil

	case pref.FloatKind:
		value := getTrimmedValue()
		if value == "" {
			return DefaultFloat32Value, false, nil
		}
		val, err := strconv.ParseFloat(value, 32)
		return pref.ValueOfFloat32(float32(val)), true, xerrors.E2012("float", value, err)

	case pref.DoubleKind:
		value := getTrimmedValue()
		if value == "" {
			return DefaultFloat64Value, false, nil
		}
		val, err := strconv.ParseFloat(value, 64)
		return pref.ValueOfFloat64(val), true, xerrors.E2012("double", value, err)

	case pref.StringKind:
		value := getValue()
		var present bool
		if value != "" {
			present = true
		} else {
			present = fd.HasPresence()
		}
		return pref.ValueOfString(value), present, nil

	case pref.BytesKind:
		value := getValue()
		var present bool
		if value != "" {
			present = true
		} else {
			present = fd.HasPresence()
		}
		return pref.ValueOfBytes([]byte(value)), present, nil

	case pref.EnumKind:
		value := getTrimmedValue()
		return parseEnumValue(fd, value)
	case pref.MessageKind:
		value := getTrimmedValue()
		msgName := fd.Message().FullName()
		switch msgName {
		case types.WellKnownMessageTimestamp:
			if value == "" {
				return DefaultTimestampValue, false, nil
			}
			// location name examples: "Asia/Shanghai" or "Asia/Chongqing".
			// NOTE(wenchy): There is no "Asia/Beijing" location name. Whoa!!! Big surprize?
			t, err := parseTimeWithLocation(locationName, value)
			if err != nil {
				return DefaultTimestampValue, true, xerrors.E2007(value, err)
			}
			// log.Debugf("timeStr: %v, unix timestamp: %v", value, t.Unix())

			ts := timestamppb.New(t)
			if err := ts.CheckValid(); err != nil {
				return DefaultTimestampValue, true, xerrors.WrapKV(err)
			}
			md := fd.Message()
			msg := dynamicpb.NewMessage(md)
			msg.Set(md.Fields().ByName("seconds"), pref.ValueOfInt64(ts.Seconds))
			msg.Set(md.Fields().ByName("nanos"), pref.ValueOfInt32(ts.Nanos))
			return pref.ValueOfMessage(msg.ProtoReflect()), true, nil

			// NOTE(wenchy): should not use ts.ProtoReflect(), as descriptor not same.
			// See more details at internal/x/xproto/build_test.go#TestCloneWellknownTypes
			// return pref.ValueOf(ts.ProtoReflect()), true, nil
		case types.WellKnownMessageDuration:
			if value == "" {
				return DefaultDurationValue, false, nil
			}
			d, err := parseDuration(value)
			if err != nil {
				return DefaultDurationValue, true, xerrors.E2008(value, err)
			}
			du := durationpb.New(d)
			if err := du.CheckValid(); err != nil {
				return DefaultDurationValue, true, xerrors.WrapKV(err)
			}
			md := fd.Message()
			msg := dynamicpb.NewMessage(md)
			msg.Set(md.Fields().ByName("seconds"), pref.ValueOfInt64(du.Seconds))
			msg.Set(md.Fields().ByName("nanos"), pref.ValueOfInt32(du.Nanos))
			return pref.ValueOfMessage(msg.ProtoReflect()), true, nil

			// NOTE(wenchy): should not use du.ProtoReflect(), as descriptor not same.
			// See more details at internal/x/xproto/build_test.go#TestCloneWellknownTypes
			// return pref.ValueOf(du.ProtoReflect()), true, nil
		case types.WellKnownMessageFraction:
			if value == "" {
				return DefaultFractionValue, false, nil
			}
			return parseFraction(fd.Message(), value)
		case types.WellKnownMessageComparator:
			if value == "" {
				return DefaultComparatorValue, false, nil
			}
			return parseComparator(fd.Message(), value)
		case types.WellKnownMessageVersion:
			if value == "" {
				return DefaultVersionValue, false, nil
			}
			return parseVersion(fd.Message(), value, fprop.GetPattern())
		default:
			return pref.Value{}, false, xerrors.Errorf("not supported message type: %s", msgName)
		}
	// case pref.GroupKind:
	// 	log.Panicf("not supported key type: %s", fd.Kind().String())
	// 	return pref.Value{}
	default:
		return pref.Value{}, false, xerrors.Errorf("not supported scalar type: %s", fd.Kind().String())
	}
}

func GetFieldDefaultValue(fd pref.FieldDescriptor) string {
	opts := fd.Options().(*descriptorpb.FieldOptions)
	fieldOpts := proto.GetExtension(opts, tableaupb.E_Field).(*tableaupb.FieldOptions)
	if fieldOpts != nil && fieldOpts.Prop != nil {
		return fieldOpts.Prop.Default
	}
	return ""
}

// ParseBool parses bool value from cell data. Floats with a decimal part of 0
// are treated as integers.
func ParseBool(value string) (bool, error) {
	// trim integer boring suffix matched by regexp `.0+$`
	if matches := types.MatchBoringInteger(value); len(matches) > 1 {
		return strconv.ParseBool(matches[1])
	}
	return strconv.ParseBool(value)
}

func parseEnumValue(fd pref.FieldDescriptor, value string) (v pref.Value, present bool, err error) {
	// return default enum value if not set
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
		return DefaultEnumValue, true, xerrors.E2006(value, ed.FullName())
	}

	// try enum value name
	evd := ed.Values().ByName(pref.Name(value))
	if evd != nil {
		return pref.ValueOfEnum(evd.Number()), true, nil
	}

	// try enum value alias
	evalue, err := enumCache.GetValueByAlias(ed, value)
	if err != nil {
		return DefaultEnumValue, false, err
	}
	return evalue, true, nil
}

func parseTimeWithLocation(locationName string, timeStr string) (time.Time, error) {
	// see https://golang.org/pkg/time/#LoadLocation
	if location, err := time.LoadLocation(locationName); err != nil {
		return time.Time{}, xerrors.WrapKV(err)
	} else {
		timeStr = strings.TrimSpace(timeStr)
		layout := ""
		if strings.Contains(timeStr, " ") {
			layout = time.DateTime
		} else {
			layout = time.DateOnly
			if !strings.Contains(timeStr, "-") {
				layout = "20060102"
			} else if strings.Contains(timeStr, "T") {
				layout = time.RFC3339
			}
		}
		t, err := time.ParseInLocation(layout, timeStr, location)
		if err != nil {
			return time.Time{}, xerrors.WrapKV(err)
		}
		return t, nil
	}
}

func parseDuration(val string) (time.Duration, error) {
	val = strings.TrimSpace(val)
	if !strings.ContainsAny(val, ":hmsµu") {
		switch len(val) {
		case 4:
			// "HHmm" -> "<HH>h<mm>m", e.g.:  "1010" -> "10h10m"
			val = val[0:2] + "h" + val[2:4] + "m"
		case 6:
			// "HHmmss" -> "<HH>h<mm>m<ss>s", e.g.: "101010" -> "10h10m10s"
			val = val[0:2] + "h" + val[2:4] + "m" + val[4:] + "s"
		default:
			return time.Duration(0), xerrors.Errorf(`invalid time format, please follow format like: "HHmmss" or "HHmm"`)
		}
	} else if strings.Contains(val, ":") {
		// TODO: check hour < 24, minute < 60, second < 60
		splits := strings.SplitN(val, ":", 3)
		switch len(splits) {
		case 2:
			// "HH:mm" -> "<HH>h<mm>m", e.g.: "10:10" -> "10h10m"
			val = splits[0] + "h" + splits[1] + "m"
		case 3:
			// "HH:mm:ss" -> "<HH>h<mm>m<ss>s", e.g.: "10:10:10" -> "10h10m10s"
			val = splits[0] + "h" + splits[1] + "m" + splits[2] + "s"
		default:
			return time.Duration(0), xerrors.Errorf(`invalid time format, please follow format like: "HH:mm:ss" or "HH:mm"`)
		}

	}

	return time.ParseDuration(val)
}

// parseFraction parses a fraction from following forms:
//   - N%: percentage, e.g.: 10%
//   - N‰: per thounsand, e.g.: 10‰
//   - N‱: per ten thounsand, e.g.: 10‱
//   - N/D: 3/4
//   - N: 3 is same to 3/1
func parseFraction(md pref.MessageDescriptor, value string) (v pref.Value, present bool, err error) {
	var numStr string
	var den int32
	if strings.HasSuffix(value, "%") {
		numStr = strings.TrimSuffix(value, "%")
		den = 100
	} else if strings.HasSuffix(value, "‰") {
		numStr = strings.TrimSuffix(value, "‰")
		den = 1000
	} else if strings.HasSuffix(value, "‱") {
		numStr = strings.TrimSuffix(value, "‱")
		den = 10000
	} else if strings.Contains(value, "/") {
		splits := strings.SplitN(value, "/", 2)
		numStr = splits[0]
		denStr := splits[1]
		den, err = parseInt32(denStr)
		if err != nil {
			return DefaultFractionValue, false, xerrors.E2019(value, err)
		}
	} else {
		numStr = value
		den = 1
	}
	num, err := parseInt32(numStr)
	if err != nil {
		return DefaultFractionValue, false, xerrors.E2019(value, err)
	}
	msg := dynamicpb.NewMessage(md)
	msg.Set(md.Fields().ByName("num"), pref.ValueOfInt32(num))
	msg.Set(md.Fields().ByName("den"), pref.ValueOfInt32(den))
	return pref.ValueOfMessage(msg.ProtoReflect()), true, nil
}

func parseComparator(md pref.MessageDescriptor, value string) (v pref.Value, present bool, err error) {
	index, err := findFirstDigitOrSignIndex(value)
	if err != nil {
		return DefaultComparatorValue, false, xerrors.E2020(value, err)
	}
	// split the string into two parts
	signStr := strings.TrimSpace(value[:index])
	fractionStr := strings.TrimSpace(value[index:])
	var sign tableaupb.Comparator_Sign
	switch signStr {
	case "==":
		sign = tableaupb.Comparator_SIGN_EQUAL
	case "!=":
		sign = tableaupb.Comparator_SIGN_NOT_EQUAL
	case "<":
		sign = tableaupb.Comparator_SIGN_LESS
	case "<=":
		sign = tableaupb.Comparator_SIGN_LESS_OR_EQUAL
	case ">":
		sign = tableaupb.Comparator_SIGN_GREATER
	case ">=":
		sign = tableaupb.Comparator_SIGN_GREATER_OR_EQUAL
	default:
		err := fmt.Errorf("unknown comparator sign: %s", signStr)
		return DefaultComparatorValue, false, xerrors.E2020(value, err)
	}

	msg := dynamicpb.NewMessage(md)
	// sign
	signFD := md.Fields().ByName("sign")
	signVal, _, err := parseEnumValue(signFD, sign.String())
	if err != nil {
		return DefaultComparatorValue, false, err
	}
	msg.Set(signFD, signVal)
	// fraction
	valueFD := md.Fields().ByName("value")
	valueMD := valueFD.Message()
	valueVal, _, err := parseFraction(valueMD, fractionStr)
	if err != nil {
		return DefaultComparatorValue, false, err
	}
	msg.Set(valueFD, valueVal)
	return pref.ValueOfMessage(msg.ProtoReflect()), true, nil
}

func parseVersion(md pref.MessageDescriptor, value string, pattern string) (v pref.Value, present bool, err error) {
	if pattern == "" {
		pattern = options.DefaultVersionPattern
	}
	// pattern
	patternSlice, err := patternToSlice(pattern)
	if err != nil {
		return DefaultVersionValue, false, err
	}
	// value
	versionStrSlice := strings.Split(value, ".")
	if len(versionStrSlice) != len(patternSlice) {
		return DefaultVersionValue, false, xerrors.E2025(value, pattern)
	}
	versionSlice := make([]uint32, 0, len(versionStrSlice))
	for i, s := range versionStrSlice {
		d, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return DefaultVersionValue, false, xerrors.E2024(value, err)
		}
		if uint32(d) > patternSlice[i] {
			return DefaultVersionValue, false, xerrors.E2025(value, pattern)
		}
		versionSlice = append(versionSlice, uint32(d))
	}

	msg := dynamicpb.NewMessage(md)
	// string form
	strFD := md.Fields().ByName("str")
	msg.Set(strFD, pref.ValueOfString(value))
	// integer form
	valFD := md.Fields().ByName("val")
	versionVal := uint64(0)
	patternSlice = append(patternSlice, 0)
	for i, v := range versionSlice {
		versionVal += uint64(v)
		if i != len(versionSlice)-1 {
			versionVal *= uint64(patternSlice[i+1]) + 1
		}
	}
	msg.Set(valFD, pref.ValueOfUint64(versionVal))
	// major.minor.patch.others
	majorFD := md.Fields().ByName("major")
	msg.Set(majorFD, pref.ValueOfUint32(versionSlice[0]))
	if len(versionSlice) > 1 {
		minorFD := md.Fields().ByName("minor")
		msg.Set(minorFD, pref.ValueOfUint32(versionSlice[1]))
	}
	if len(versionSlice) > 2 {
		patchFD := md.Fields().ByName("patch")
		msg.Set(patchFD, pref.ValueOfUint32(versionSlice[2]))
	}
	if len(versionSlice) > 3 {
		patchFD := md.Fields().ByName("others")
		for i := 3; i < len(versionSlice); i++ {
			msg.Mutable(patchFD).List().Append(pref.ValueOfUint32(versionSlice[i]))
		}
	}
	return pref.ValueOfMessage(msg.ProtoReflect()), true, nil
}

var versionPatterns = &versionPatternCache{
	cache: map[string][]uint32{},
}

type versionPatternCache struct {
	sync.RWMutex
	cache map[string][]uint32 // pattern str -> pattern slice
}

func patternToSlice(pattern string) ([]uint32, error) {
	versionPatterns.RLock()
	v, ok := versionPatterns.cache[pattern]
	versionPatterns.RUnlock()
	if ok {
		return v, nil
	}
	versionPatterns.Lock()
	defer versionPatterns.Unlock()
	patternStrSlice := strings.Split(pattern, ".")
	patternSlice := make([]uint32, 0, len(patternStrSlice))
	for _, s := range patternStrSlice {
		d, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return nil, xerrors.E2024(pattern, err)
		}
		patternSlice = append(patternSlice, uint32(d))
	}
	product := uint64(1)
	for _, v := range patternSlice {
		multiplier := uint64(v) + 1
		if product > math.MaxUint64/multiplier {
			return nil, xerrors.E2024(pattern, xerrors.Errorf("product of all pattern decimals overflow uint64"))
		}
		product *= multiplier
	}
	versionPatterns.cache[pattern] = patternSlice
	return patternSlice, nil
}

func findFirstDigitOrSignIndex(s string) (int, error) {
	for i, char := range s {
		if unicode.IsDigit(char) || char == '-' || char == '+' {
			return i, nil
		}
	}
	return 0, fmt.Errorf("number part not found")
}

func parseInt32(value string) (int32, error) {
	val, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, err
	}
	if val < math.MinInt32 || val > math.MaxInt32 {
		return 0, xerrors.E2000("int32", value, math.MinInt32, math.MaxInt32)
	}
	return int32(val), nil
}
