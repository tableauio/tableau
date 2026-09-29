package store

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestProtoJSONMarshalsNestedTimestamps(t *testing.T) {
	newConf := func(name string, year int) *unittestpb.PatchMergeConf {
		return &unittestpb.PatchMergeConf{
			Name: name,
			Time: &unittestpb.PatchMergeConf_Time{
				Start: timestamppb.New(time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)),
			},
		}
	}
	message := &unittestpb.JsonUtilTestData{
		NormalField: newConf("nested", 2022),
		ListField: []*unittestpb.PatchMergeConf{
			newConf("list", 2023),
		},
		MapField: map[int32]*unittestpb.PatchMergeConf{
			2025: newConf("map", 2025),
		},
	}

	got, err := MarshalToJSON(message, &MarshalOptions{
		EmitTimezones: true,
		LocationName:  "Asia/Shanghai",
		UseProtoNames: true,
	})
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"normal_field":{"name":"nested","time":{"start":"2022-01-01T08:00:00+08:00"}},
		"list_field":[{"name":"list","time":{"start":"2023-01-01T08:00:00+08:00"}}],
		"map_field":{"2025":{"name":"map","time":{"start":"2025-01-01T08:00:00+08:00"}}}
	}`, string(got))
}

func TestProtoJSONMarshalsTimestampFractions(t *testing.T) {
	tests := []struct {
		name  string
		nanos int32
		want  string
	}{
		{name: "none", nanos: 0, want: `"2022-01-01T08:00:00+08:00"`},
		{name: "nanoseconds", nanos: 1, want: `"2022-01-01T08:00:00.000000001+08:00"`},
		{name: "shortened", nanos: 100_000, want: `"2022-01-01T08:00:00.0001+08:00"`},
		{name: "milliseconds", nanos: 1_000_000, want: `"2022-01-01T08:00:00.001+08:00"`},
		{name: "four digits", nanos: 123_400_000, want: `"2022-01-01T08:00:00.1234+08:00"`},
		{name: "full precision", nanos: 123_456_789, want: `"2022-01-01T08:00:00.123456789+08:00"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := MarshalToJSON(&timestamppb.Timestamp{
				Seconds: 1_640_995_200,
				Nanos:   test.nanos,
			}, &MarshalOptions{
				EmitTimezones: true,
				LocationName:  "Asia/Shanghai",
			})
			require.NoError(t, err)
			assert.Equal(t, test.want, string(got))
		})
	}
}

func TestLoadLocationTracksLocal(t *testing.T) {
	original := time.Local
	t.Cleanup(func() { time.Local = original })

	first := time.FixedZone("first", int(time.Hour/time.Second))
	time.Local = first
	got, err := loadLocation("Local")
	require.NoError(t, err)
	require.Same(t, first, got)

	second := time.FixedZone("second", 2*int(time.Hour/time.Second))
	time.Local = second
	got, err = loadLocation("Local")
	require.NoError(t, err)
	require.Same(t, second, got)
}

func TestProtoJSONFallsBackToUTCOutsideTimestampRange(t *testing.T) {
	tests := []struct {
		name     string
		message  *timestamppb.Timestamp
		location *time.Location
		want     string
	}{
		{
			name:     "minimum underflow",
			message:  &timestamppb.Timestamp{Seconds: minTimestampSeconds},
			location: time.FixedZone("minus-one", -int(time.Hour/time.Second)),
			want:     `"0001-01-01T00:00:00Z"`,
		},
		{
			name: "maximum overflow",
			message: &timestamppb.Timestamp{
				Seconds: maxTimestampSeconds,
				Nanos:   999_999_999,
			},
			location: time.FixedZone("plus-one", int(time.Hour/time.Second)),
			want:     `"9999-12-31T23:59:59.999999999Z"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := (protoJSONOptions{
				MarshalOptions: protojson.MarshalOptions{},
				location:       test.location,
			}).Marshal(test.message)
			require.NoError(t, err)
			assert.Equal(t, test.want, string(got))
		})
	}
}

func TestProtoJSONRejectsInvalidTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		message *timestamppb.Timestamp
		want    string
	}{
		{
			name:    "seconds",
			message: &timestamppb.Timestamp{Seconds: maxTimestampSeconds + 1},
			want:    "seconds out of range",
		},
		{
			name:    "nanos",
			message: &timestamppb.Timestamp{Nanos: 1_000_000_000},
			want:    "nanos out of range",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := (protoJSONOptions{
				MarshalOptions: protojson.MarshalOptions{},
				location:       time.UTC,
			}).Marshal(test.message)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.want)
		})
	}
}

func TestProtoJSONPreservesNilAndOrdinaryStrings(t *testing.T) {
	t.Run("unpopulated timestamp", func(t *testing.T) {
		got, err := MarshalToJSON(&unittestpb.PatchMergeConf_Time{}, &MarshalOptions{
			EmitTimezones:   true,
			LocationName:    "Asia/Shanghai",
			EmitUnpopulated: true,
			UseProtoNames:   true,
		})
		require.NoError(t, err)
		assert.Equal(t, `{"start":null,"expiry":null}`, string(got))
	})

	t.Run("ordinary RFC3339 string", func(t *testing.T) {
		message := &unittestpb.PatchMergeConf{
			Name: "2022-01-01T00:00:00Z",
			Time: &unittestpb.PatchMergeConf_Time{
				Start: timestamppb.New(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
			},
		}
		got, err := MarshalToJSON(message, &MarshalOptions{
			EmitTimezones: true,
			LocationName:  "Asia/Shanghai",
			UseProtoNames: true,
		})
		require.NoError(t, err)
		assert.Equal(t, `{"name":"2022-01-01T00:00:00Z","time":{"start":"2022-01-01T08:00:00+08:00"}}`, string(got))
	})
}

func TestProtoJSONPreservesNilMessageValues(t *testing.T) {
	message := &unittestpb.JsonUtilTestData{
		ListField: []*unittestpb.PatchMergeConf{nil},
		MapField:  map[int32]*unittestpb.PatchMergeConf{1: nil},
	}
	got, err := MarshalToJSON(message, &MarshalOptions{
		EmitTimezones: true,
		LocationName:  "Asia/Shanghai",
		UseProtoNames: true,
	})
	require.NoError(t, err)
	assert.Equal(t, `{"list_field":[{}],"map_field":{"1":{}}}`, string(got))
}

func TestProtoJSONLeavesTimestampInAnyAsUTC(t *testing.T) {
	packed, err := anypb.New(timestamppb.New(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)))
	require.NoError(t, err)

	got, err := MarshalToJSON(packed, &MarshalOptions{
		EmitTimezones: true,
		LocationName:  "Asia/Shanghai",
	})
	require.NoError(t, err)
	assert.Equal(t, `{"@type":"type.googleapis.com/google.protobuf.Timestamp","value":"2022-01-01T00:00:00Z"}`, string(got))
}

func TestProtoJSONMarshalsTimestampExtension(t *testing.T) {
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:       proto.String("timestamp_extension.proto"),
		Package:    proto.String("store.test"),
		Syntax:     proto.String("proto2"),
		Dependency: []string{"google/protobuf/timestamp.proto"},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Extendable"),
			ExtensionRange: []*descriptorpb.DescriptorProto_ExtensionRange{{
				Start: proto.Int32(100),
				End:   proto.Int32(101),
			}},
		}},
		Extension: []*descriptorpb.FieldDescriptorProto{{
			Name:     proto.String("timestamp_ext"),
			Extendee: proto.String(".store.test.Extendable"),
			Number:   proto.Int32(100),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			TypeName: proto.String(".google.protobuf.Timestamp"),
		}},
	}, protoregistry.GlobalFiles)
	require.NoError(t, err)

	message := dynamicpb.NewMessage(file.Messages().Get(0))
	extension := dynamicpb.NewExtensionType(file.Extensions().Get(0))
	timestamp := timestamppb.New(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC))
	proto.SetExtension(message, extension, timestamp.ProtoReflect())

	got, err := MarshalToJSON(message, &MarshalOptions{
		EmitTimezones: true,
		LocationName:  "Asia/Shanghai",
	})
	require.NoError(t, err)
	assert.Equal(t, `{"[store.test.timestamp_ext]":"2022-01-01T08:00:00+08:00"}`, string(got))
}

func TestCollectTimestampTypesHandlesRecursiveMessages(t *testing.T) {
	tests := []struct {
		name             string
		includeTimestamp bool
	}{
		{name: "without timestamp"},
		{name: "with timestamp", includeTimestamp: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fields := []*descriptorpb.FieldDescriptorProto{{
				Name:     proto.String("child"),
				JsonName: proto.String("child"),
				Number:   proto.Int32(1),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
				TypeName: proto.String(".store.recursive.Node"),
			}}
			if test.includeTimestamp {
				fields = append(fields, &descriptorpb.FieldDescriptorProto{
					Name:     proto.String("time"),
					JsonName: proto.String("time"),
					Number:   proto.Int32(2),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
					TypeName: proto.String(".google.protobuf.Timestamp"),
				})
			}
			file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
				Name:       proto.String("recursive.proto"),
				Package:    proto.String("store.recursive"),
				Syntax:     proto.String("proto3"),
				Dependency: []string{"google/protobuf/timestamp.proto"},
				MessageType: []*descriptorpb.DescriptorProto{{
					Name:  proto.String("Node"),
					Field: fields,
				}},
			}, protoregistry.GlobalFiles)
			require.NoError(t, err)

			matches := make(map[protoreflect.FullName]bool)
			assert.Equal(t, test.includeTimestamp, collectTimestampTypes(file.Messages().Get(0), matches))
			assert.Equal(t, test.includeTimestamp, matches["store.recursive.Node"])
		})
	}
}

func TestProtoJSONMarshalsStableCompactAndPrettyOutput(t *testing.T) {
	message := &unittestpb.PatchMergeConf{
		Name: "comma,  colon:  value",
		Time: &unittestpb.PatchMergeConf_Time{
			Start: timestamppb.New(time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
	}
	tests := []struct {
		name   string
		pretty bool
		want   string
	}{
		{
			name: "compact",
			want: `{"name":"comma,  colon:  value","time":{"start":"2022-01-01T08:00:00+08:00"}}`,
		},
		{
			name:   "pretty",
			pretty: true,
			want: "{\n" +
				`    "name": "comma,  colon:  value",` + "\n" +
				`    "time": {` + "\n" +
				`        "start": "2022-01-01T08:00:00+08:00"` + "\n" +
				"    }\n" +
				"}",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := MarshalToJSON(message, &MarshalOptions{
				EmitTimezones: true,
				LocationName:  "Asia/Shanghai",
				UseProtoNames: true,
				Pretty:        test.pretty,
			})
			require.NoError(t, err)
			assert.Equal(t, test.want, string(got))
		})
	}
}

func TestProtoJSONHonorsAllowPartial(t *testing.T) {
	message := newRequiredTimestampMessage(t)
	options := protoJSONOptions{
		MarshalOptions: protojson.MarshalOptions{AllowPartial: true},
		location:       time.FixedZone("plus-one", int(time.Hour/time.Second)),
	}
	got, err := options.Marshal(message)
	require.NoError(t, err)
	assert.JSONEq(t, `{"time":"1970-01-01T01:00:00+01:00"}`, string(got))

	options.AllowPartial = false
	_, err = options.Marshal(message)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "required") && strings.Contains(err.Error(), "name"))
}

func newRequiredTimestampMessage(t *testing.T) proto.Message {
	t.Helper()
	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:       proto.String("required_timestamp.proto"),
		Package:    proto.String("store.test"),
		Syntax:     proto.String("proto2"),
		Dependency: []string{"google/protobuf/timestamp.proto"},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("RequiredTimestamp"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:     proto.String("name"),
					JsonName: proto.String("name"),
					Number:   proto.Int32(1),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_REQUIRED.Enum(),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				},
				{
					Name:     proto.String("time"),
					JsonName: proto.String("time"),
					Number:   proto.Int32(2),
					Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
					TypeName: proto.String(".google.protobuf.Timestamp"),
				},
			},
		}},
	}, protoregistry.GlobalFiles)
	require.NoError(t, err)

	message := dynamicpb.NewMessage(file.Messages().Get(0))
	timeField := message.Descriptor().Fields().ByName(protoreflect.Name("time"))
	message.Set(timeField, protoreflect.ValueOfMessage((&timestamppb.Timestamp{}).ProtoReflect()))
	return message
}
