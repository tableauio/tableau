package xproto

import (
	"fmt"
	"math"
	"testing"

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
			wantV:       pref.ValueOfUint64(18446744073709551615 ),
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
			gotV, gotPresent, err := ParseFieldValue(tt.args.fd, tt.args.rawValue, tt.args.locationName)
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
