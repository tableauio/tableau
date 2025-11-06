package xproto

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	pref "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestParseFieldValue(t *testing.T) {
	var int32Value wrapperspb.Int32Value
	int32ValueFd := int32Value.ProtoReflect().Descriptor().Fields().ByNumber(1)
	var uint32Value wrapperspb.UInt32Value
	uint32ValueFd := uint32Value.ProtoReflect().Descriptor().Fields().ByNumber(1)
	var int64Value wrapperspb.Int64Value
	int64ValueFd := int64Value.ProtoReflect().Descriptor().Fields().ByNumber(1)
	var uint64Value wrapperspb.UInt64Value
	uint64ValueFd := uint64Value.ProtoReflect().Descriptor().Fields().ByNumber(1)
	var boolValue wrapperspb.BoolValue
	boolValueFd := boolValue.ProtoReflect().Descriptor().Fields().ByNumber(1)
	var floatValue wrapperspb.FloatValue
	floatValueFd := floatValue.ProtoReflect().Descriptor().Fields().ByNumber(1)
	var doubleValue wrapperspb.DoubleValue
	doubleValueFd := doubleValue.ProtoReflect().Descriptor().Fields().ByNumber(1)
	var stringValue wrapperspb.StringValue
	stringValueFd := stringValue.ProtoReflect().Descriptor().Fields().ByNumber(1)
	var bytesValue wrapperspb.BytesValue
	bytesValueFd := bytesValue.ProtoReflect().Descriptor().Fields().ByNumber(1)

	type args struct {
		fd           pref.FieldDescriptor
		rawValue     string
		locationName string
	}
	tests := []struct {
		name        string
		args        args
		wantV       pref.Value
		wantPresent bool
		wantErr     bool
	}{
		{
			name: "int32",
			args: args{
				fd:       int32ValueFd,
				rawValue: "-100",
			},
			wantV:       pref.ValueOfInt32(-100),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "int32 overflow < math.MinInt32",
			args: args{
				fd:       int32ValueFd,
				rawValue: fmt.Sprintf("%d", math.MinInt64),
			},
			wantV:       DefaultInt32Value,
			wantPresent: false,
			wantErr:     true,
		},
		{
			name: "int32 overflow > math.MaxInt32",
			args: args{
				fd:       int32ValueFd,
				rawValue: fmt.Sprintf("%d", math.MaxInt64),
			},
			wantV:       DefaultInt32Value,
			wantPresent: false,
			wantErr:     true,
		},
		{
			name: "uint32",
			args: args{
				fd:       uint32ValueFd,
				rawValue: "100",
			},
			wantV:       pref.ValueOfUint32(100),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "uint32 overflow < 0",
			args: args{
				fd:       uint32ValueFd,
				rawValue: "-1",
			},
			wantV:       DefaultUint32Value,
			wantPresent: false,
			wantErr:     true,
		},
		{
			name: "uint32 overflow > math.MaxInt32",
			args: args{
				fd:       uint32ValueFd,
				rawValue: fmt.Sprintf("%d", uint64(math.MaxUint64)),
			},
			wantV:       DefaultUint32Value,
			wantPresent: false,
			wantErr:     true,
		},
		{
			name: "int64 max",
			args: args{
				fd:       int64ValueFd,
				rawValue: "9223372036854775807",
			},
			wantV:       pref.ValueOfInt64(9223372036854775807),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "int64 min",
			args: args{
				fd:       int64ValueFd,
				rawValue: "-9223372036854775807",
			},
			wantV:       pref.ValueOfInt64(-9223372036854775807),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "int64 overflow < math.MinInt64",
			args: args{
				fd:       int64ValueFd,
				rawValue: fmt.Sprintf("-%d", uint64(math.MaxUint64)),
			},
			wantV:       DefaultInt64Value,
			wantPresent: false,
			wantErr:     true,
		},
		{
			name: "int64 overflow > math.MaxInt64",
			args: args{
				fd:       int64ValueFd,
				rawValue: fmt.Sprintf("%d", uint64(math.MaxUint64)),
			},
			wantV:       DefaultInt64Value,
			wantPresent: false,
			wantErr:     true,
		},
		{
			name: "uint64 max",
			args: args{
				fd:       uint64ValueFd,
				rawValue: "18446744073709551615 ",
			},
			wantV:       pref.ValueOfUint64(18446744073709551615),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "uint64 overflow < 0",
			args: args{
				fd:       uint64ValueFd,
				rawValue: "-1",
			},
			wantV:       DefaultUint64Value,
			wantPresent: false,
			wantErr:     true,
		},
		{
			name: "uint64 overflow > math.MaxUint64",
			args: args{
				fd:       uint64ValueFd,
				rawValue: fmt.Sprintf("10%d", uint64(math.MaxUint64)),
			},
			wantV:       DefaultUint64Value,
			wantPresent: false,
			wantErr:     true,
		},
		{
			name: "bool",
			args: args{
				fd:       boolValueFd,
				rawValue: "true",
			},
			wantV:       pref.ValueOfBool(true),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "bool",
			args: args{
				fd:       boolValueFd,
				rawValue: "True",
			},
			wantV:       pref.ValueOfBool(true),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "bool",
			args: args{
				fd:       boolValueFd,
				rawValue: "TRUE",
			},
			wantV:       pref.ValueOfBool(true),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "bool",
			args: args{
				fd:       boolValueFd,
				rawValue: "1",
			},
			wantV:       pref.ValueOfBool(true),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "bool",
			args: args{
				fd:       boolValueFd,
				rawValue: "false",
			},
			wantV:       pref.ValueOfBool(false),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "bool",
			args: args{
				fd:       boolValueFd,
				rawValue: "False",
			},
			wantV:       pref.ValueOfBool(false),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "bool",
			args: args{
				fd:       boolValueFd,
				rawValue: "FALSE",
			},
			wantV:       pref.ValueOfBool(false),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "bool",
			args: args{
				fd:       boolValueFd,
				rawValue: "0",
			},
			wantV:       pref.ValueOfBool(false),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "bool",
			args: args{
				fd:       boolValueFd,
				rawValue: "xxx", // invalid syntax
			},
			wantV:       DefaultBoolValue,
			wantPresent: false,
			wantErr:     true,
		},
		{
			name: "float32",
			args: args{
				fd:       floatValueFd,
				rawValue: "3.14",
			},
			wantV:       pref.ValueOfFloat32(3.14),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "float32 empty raw value",
			args: args{
				fd:       floatValueFd,
				rawValue: "",
			},
			wantV:       DefaultFloat32Value,
			wantPresent: false,
			wantErr:     false,
		},
		{
			name: "float64",
			args: args{
				fd:       doubleValueFd,
				rawValue: "3.14",
			},
			wantV:       pref.ValueOfFloat64(3.14),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "float64 empty raw value",
			args: args{
				fd:       doubleValueFd,
				rawValue: "",
			},
			wantV:       DefaultFloat64Value,
			wantPresent: false,
			wantErr:     false,
		},
		{
			name: "string",
			args: args{
				fd:       stringValueFd,
				rawValue: "test",
			},
			wantV:       pref.ValueOfString("test"),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "string empty raw value",
			args: args{
				fd:       stringValueFd,
				rawValue: "",
			},
			wantV:       DefaultStringValue,
			wantPresent: false,
			wantErr:     false,
		},
		{
			name: "bytes",
			args: args{
				fd:       bytesValueFd,
				rawValue: "test",
			},
			wantV:       pref.ValueOfBytes([]byte("test")),
			wantPresent: true,
			wantErr:     false,
		},
		{
			name: "bytes empty raw value",
			args: args{
				fd:       bytesValueFd,
				rawValue: "",
			},
			wantV:       DefaultBytesValue,
			wantPresent: false,
			wantErr:     false,
		},
		// TODO: add cases for enum and well-known messages
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotV, gotPresent, err := ParseFieldValue(tt.args.fd, tt.args.rawValue, tt.args.locationName, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFieldValue() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !gotV.Equal(tt.wantV) {
				t.Errorf("ParseFieldValue() gotV = %v, want %v", gotV, tt.wantV)
			}
			if gotPresent != tt.wantPresent {
				t.Errorf("ParseFieldValue() gotPresent = %v, want %v", gotPresent, tt.wantPresent)
			}
		})
	}
}

func Test_parseFraction(t *testing.T) {
	msg := &tableaupb.Fraction{}
	md := msg.ProtoReflect().Descriptor()
	type args struct {
		value string
	}
	tests := []struct {
		name    string
		args    args
		wantF   *tableaupb.Fraction
		wantErr bool
	}{
		{
			name: "percentage",
			args: args{
				value: "10%",
			},
			wantF: &tableaupb.Fraction{
				Num: 10,
				Den: 100,
			},
		},
		{
			name: "per-thounsand",
			args: args{
				value: "10‰",
			},
			wantF: &tableaupb.Fraction{
				Num: 10,
				Den: 1000,
			},
		},
		{
			name: "per-ten-thounsand",
			args: args{
				value: "10‱",
			},
			wantF: &tableaupb.Fraction{
				Num: 10,
				Den: 10000,
			},
		},
		{
			name: "num-den",
			args: args{
				value: "3/4",
			},
			wantF: &tableaupb.Fraction{
				Num: 3,
				Den: 4,
			},
		},
		{
			name: "only-num",
			args: args{
				value: "10",
			},
			wantF: &tableaupb.Fraction{
				Num: 10,
				Den: 1,
			},
		},
		{
			name: "negative",
			args: args{
				value: "-6/10",
			},
			wantF: &tableaupb.Fraction{
				Num: -6,
				Den: 10,
			},
		},
		{
			name: "positive",
			args: args{
				value: "+6/10",
			},
			wantF: &tableaupb.Fraction{
				Num: +6,
				Den: 10,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotF, _, err := parseFraction(md, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseFraction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && !gotF.Equal(pref.ValueOfMessage(tt.wantF.ProtoReflect())) {
				t.Errorf("parseFraction() = %v, want %v", gotF, tt.wantF)
			}
		})
	}
}

func Test_parseComparator(t *testing.T) {
	msg := &tableaupb.Comparator{}
	md := msg.ProtoReflect().Descriptor()
	type args struct {
		value string
	}
	tests := []struct {
		name    string
		args    args
		wantC   *tableaupb.Comparator
		wantErr bool
	}{
		{
			name: "equal",
			args: args{
				value: "==10%",
			},
			wantC: &tableaupb.Comparator{
				Sign: tableaupb.Comparator_SIGN_EQUAL,
				Value: &tableaupb.Fraction{
					Num: 10,
					Den: 100,
				},
			},
		},
		{
			name: "not-equal",
			args: args{
				value: "!=10",
			},
			wantC: &tableaupb.Comparator{
				Sign: tableaupb.Comparator_SIGN_NOT_EQUAL,
				Value: &tableaupb.Fraction{
					Num: 10,
					Den: 1,
				},
			},
		},
		{
			name: "less",
			args: args{
				value: "<10%",
			},
			wantC: &tableaupb.Comparator{
				Sign: tableaupb.Comparator_SIGN_LESS,
				Value: &tableaupb.Fraction{
					Num: 10,
					Den: 100,
				},
			},
		},
		{
			name: "less-or-equal",
			args: args{
				value: "<=10%",
			},
			wantC: &tableaupb.Comparator{
				Sign: tableaupb.Comparator_SIGN_LESS_OR_EQUAL,
				Value: &tableaupb.Fraction{
					Num: 10,
					Den: 100,
				},
			},
		},
		{
			name: "greater-with-space",
			args: args{
				value: "> 10%",
			},
			wantC: &tableaupb.Comparator{
				Sign: tableaupb.Comparator_SIGN_GREATER,
				Value: &tableaupb.Fraction{
					Num: 10,
					Den: 100,
				},
			},
		},
		{
			name: "greater-or-equal-with-two-space",
			args: args{
				value: ">=  10%",
			},
			wantC: &tableaupb.Comparator{
				Sign: tableaupb.Comparator_SIGN_GREATER_OR_EQUAL,
				Value: &tableaupb.Fraction{
					Num: 10,
					Den: 100,
				},
			},
		},
		{
			name: "negative",
			args: args{
				value: ">=-10%",
			},
			wantC: &tableaupb.Comparator{
				Sign: tableaupb.Comparator_SIGN_GREATER_OR_EQUAL,
				Value: &tableaupb.Fraction{
					Num: -10,
					Den: 100,
				},
			},
		},
		{
			name: "positive",
			args: args{
				value: ">=+10%",
			},
			wantC: &tableaupb.Comparator{
				Sign: tableaupb.Comparator_SIGN_GREATER_OR_EQUAL,
				Value: &tableaupb.Fraction{
					Num: +10,
					Den: 100,
				},
			},
		},
		{
			name: "invalid",
			args: args{
				value: ">==10%",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotC, _, err := parseComparator(md, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseComparator() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && !gotC.Equal(pref.ValueOfMessage(tt.wantC.ProtoReflect())) {
				t.Errorf("parseComparator() = %v, want %v", gotC, tt.wantC)
			}
		})
	}
}

func Test_parseVersion(t *testing.T) {
	msg := &tableaupb.Version{}
	md := msg.ProtoReflect().Descriptor()
	type args struct {
		value   string
		pattern string
	}
	tests := []struct {
		name    string
		args    args
		wantV   *tableaupb.Version
		wantErr bool
		err     error
	}{
		{
			name: "default pattern",
			args: args{
				value: "1.2.3",
			},
			wantV: &tableaupb.Version{
				Str:   "1.2.3",
				Val:   1<<16 | 2<<8 | 3, // 66051
				Major: 1,
				Minor: 2,
				Patch: 3,
			},
		},
		{
			name: "custom pattern",
			args: args{
				pattern: "99.999.99.999",
				value:   "12.345.67.890",
			},
			wantV: &tableaupb.Version{
				Str:    "12.345.67.890",
				Val:    1234567890,
				Major:  12,
				Minor:  345,
				Patch:  67,
				Others: []uint32{890},
			},
		},
		{
			name: "no dot in pattern",
			args: args{
				pattern: "65535",
				value:   "1024",
			},
			wantV: &tableaupb.Version{
				Str:   "1024",
				Val:   1024,
				Major: 1024,
			},
		},
		{
			name: "version mismatches pattern",
			args: args{
				pattern: "255.255.255",
				value:   "1.0.0.0",
			},
			wantErr: true,
			err:     xerrors.ErrE2025,
		},
		{
			name: "negative pattern decimal",
			args: args{
				pattern: "255.-255.255",
				value:   "1.0.0",
			},
			wantErr: true,
			err:     xerrors.ErrE2024,
		},
		{
			name: "negative version decimal",
			args: args{
				pattern: "255.255.255",
				value:   "1.-1.0",
			},
			wantErr: true,
			err:     xerrors.ErrE2024,
		},
		{
			name: "pattern decimal max uint32",
			args: args{
				pattern: fmt.Sprintf("255.255.%d", math.MaxUint32),
				value:   "0.2.0",
			},
			wantV: &tableaupb.Version{
				Str:   "0.2.0",
				Val:   2 * (math.MaxUint32 + 1),
				Minor: 2,
			},
		},
		{
			name: "pattern decimal exceeds max uint32",
			args: args{
				pattern: fmt.Sprintf("255.255.%d", math.MaxUint32+1),
				value:   "1.0.0",
			},
			wantErr: true,
			err:     xerrors.ErrE2024,
		},
		{
			name: "pattern decimal product exceeds max uint64",
			args: args{
				pattern: fmt.Sprintf("%d.%d.%d", math.MaxUint32, math.MaxUint32, math.MaxUint32),
				value:   "1.0.0",
			},
			wantErr: true,
			err:     xerrors.ErrE2024,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotV, _, err := parseVersion(md, tt.args.value, tt.args.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && !gotV.Equal(pref.ValueOfMessage(tt.wantV.ProtoReflect())) {
				t.Errorf("parseVersion() = %v, want %v", gotV, tt.wantV)
			}
			if err != nil {
				require.ErrorIs(t, err, tt.err)
			}
		})
	}
}
