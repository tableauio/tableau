package xproto

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/types"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestParseProtos(t *testing.T) {
	type args struct {
		ImportPaths []string
		filenames   []string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{
			name: "test1",
			args: args{
				ImportPaths: []string{
					"../../proto", // tableau
				},
				filenames: []string{
					"tableau/protobuf/unittest/unittest.proto",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := ParseProtos(tt.args.ImportPaths, tt.args.filenames...)
			if err != nil {
				t.Errorf("parseProtos() error = %v", err)
			}
			t.Logf("parsed proto files: %+v", files)
		})
	}
}

func TestNewFiles(t *testing.T) {
	type args struct {
		protoPaths        []string
		protoFiles        []string
		excludeProtoFiles []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "test1",
			args: args{
				protoPaths: []string{
					"../../proto", // tableau
				},
				protoFiles: []string{
					"../../proto/tableau/protobuf/unittest/*.proto",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewFiles(tt.args.protoPaths, tt.args.protoFiles, tt.args.excludeProtoFiles...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_extractTypeInfosFromMessage(t *testing.T) {
	desc1, err := protoregistry.GlobalFiles.FindDescriptorByName("unittest.Item")
	if err != nil {
		t.Fatalf("descriptor not found")
	}
	md1 := desc1.(protoreflect.MessageDescriptor)
	desc2, err := protoregistry.GlobalFiles.FindDescriptorByName("unittest.Target")
	if err != nil {
		t.Fatalf("descriptor not found")
	}
	md2 := desc2.(protoreflect.MessageDescriptor)
	type args struct {
		md protoreflect.MessageDescriptor
	}
	tests := []struct {
		name string
		args args
		want *TypeInfos
	}{
		{
			name: "simple message",
			args: args{
				md: md1,
			},
			want: &TypeInfos{
				protoPackage: "unittest",
				infos: map[protoreflect.FullName]*TypeInfo{
					"unittest.Item": {
						FullName:             "unittest.Item",
						ParentFilename:       "tableau/protobuf/unittest/common.proto",
						Kind:                 types.MessageKind,
						FirstFieldOptionName: "ID",
					},
				},
			},
		},
		{
			name: "nested message",
			args: args{
				md: md2,
			},
			want: &TypeInfos{
				protoPackage: "unittest",
				infos: map[protoreflect.FullName]*TypeInfo{
					"unittest.Target": {
						FullName:             "unittest.Target",
						ParentFilename:       "tableau/protobuf/unittest/common.proto",
						Kind:                 types.MessageKind,
						FirstFieldOptionName: "Type",
					},

					"unittest.Target.Type": {
						FullName:       "unittest.Target.Type",
						ParentFilename: "tableau/protobuf/unittest/common.proto",
						Kind:           types.EnumKind,
					},
					"unittest.Target.Pvp": {
						FullName:       "unittest.Target.Pvp",
						ParentFilename: "tableau/protobuf/unittest/common.proto",
						Kind:           types.MessageKind,
					},
					"unittest.Target.Pve": {
						FullName:             "unittest.Target.Pve",
						ParentFilename:       "tableau/protobuf/unittest/common.proto",
						Kind:                 types.MessageKind,
						FirstFieldOptionName: "Mission",
					},
					"unittest.Target.Pve.Mission": {
						FullName:       "unittest.Target.Pve.Mission",
						ParentFilename: "tableau/protobuf/unittest/common.proto",
						Kind:           types.MessageKind,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewTypeInfos("unittest")
			extractTypeInfosFromMessage(tt.args.md, got)
			assert.Equal(t, tt.want, got, "extractTypeInfosFromMessage")
		})
	}
}

func TestGetAllTypeInfo(t *testing.T) {
	type args struct {
		files        *protoregistry.Files
		protoPackage string
	}
	tests := []struct {
		name string
		args args
		want *TypeInfos
	}{
		{
			name: "test1",
			args: args{
				files:        protoregistry.GlobalFiles,
				protoPackage: "unittest",
			},
			want: &TypeInfos{
				protoPackage: "unittest",
				infos: map[protoreflect.FullName]*TypeInfo{
					"unittest.Item": {
						FullName:             "unittest.Item",
						ParentFilename:       "tableau/protobuf/unittest/common.proto",
						Kind:                 types.MessageKind,
						FirstFieldOptionName: "ID",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetAllTypeInfo(tt.args.files, tt.args.protoPackage); !reflect.DeepEqual(got.infos["unittest.Item"], tt.want.infos["unittest.Item"]) {
				t.Errorf("GetAllTypeInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTypeInfos_Get(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		x    *TypeInfos
		args args
		want *TypeInfo
	}{
		{
			name: "test1",
			x:    GetAllTypeInfo(protoregistry.GlobalFiles, "unittest"),
			args: args{
				name: ".Item",
			},
			want: &TypeInfo{
				FullName:             "unittest.Item",
				ParentFilename:       "tableau/protobuf/unittest/common.proto",
				Kind:                 types.MessageKind,
				FirstFieldOptionName: "ID",
			},
		},
		{
			name: "test2",
			x:    GetAllTypeInfo(protoregistry.GlobalFiles, "unittest"),
			args: args{
				name: "unittest.Item",
			},
			want: &TypeInfo{
				FullName:             "unittest.Item",
				ParentFilename:       "tableau/protobuf/unittest/common.proto",
				Kind:                 types.MessageKind,
				FirstFieldOptionName: "ID",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.x.Get(tt.args.name); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("TypeInfos.Get() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCloneWellknownTypes(t *testing.T) {
	importPaths := []string{
		"../../proto", // tableau
	}
	filenames := []string{
		"tableau/protobuf/unittest/unittest.proto",
	}
	files, err := ParseProtos(importPaths, filenames...)
	if err != nil {
		t.Errorf("parseProtos() error = %v", err)
	}
	// t.Logf("parsed proto files: %+v", files)
	// timestampDesc, err := files.FindDescriptorByName(protoreflect.FullName("google.protobuf.Timestamp"))
	timestampDesc, err := files.FindDescriptorByName(protoreflect.FullName("unittest.PatchMergeConf.Time"))
	require.NoError(t, err)

	// Assert to MessageDescriptor
	md := timestampDesc.(protoreflect.MessageDescriptor)

	generatedTsMd := timestamppb.New(time.Now()).ProtoReflect().Descriptor()
	// Create a new dynamic message
	protomsg := dynamicpb.NewMessage(md)

	// Set the seconds and nanos fields for the Timestamp
	now := time.Now()
	seconds := now.Unix()
	nanos := int32(now.Nanosecond())
	tsFd := md.Fields().ByName("start")
	dynamicTsMd := tsFd.Message()
	tsMsg := protomsg.Mutable(tsFd).Message()
	// Set the fields using reflection
	tsMsg.Set(dynamicTsMd.Fields().ByName("seconds"), protoreflect.ValueOf(seconds))
	tsMsg.Set(dynamicTsMd.Fields().ByName("nanos"), protoreflect.ValueOf(nanos))
	if generatedTsMd != dynamicTsMd {
		t.Logf("WARNING: timestamp descriptors from timestamppb.New() and dynamically generated by protoparse are not equal")
	}
	clonedMsg1 := proto.Clone(protomsg)
	t.Logf("clonedMsg: %v", clonedMsg1)

	// generated descriptor from [timestamp.pb.go](https://github.com/protocolbuffers/protobuf-go/blob/master/types/known/timestamppb/timestamp.pb.go)
	ts := timestamppb.New(time.Now())
	if generatedTsMd != ts.ProtoReflect().Descriptor() {
		t.Fatalf("timestamp descriptors both from timestamppb.New() should be equal")
	}
	protomsg.Set(tsFd, protoreflect.ValueOf(ts.ProtoReflect()))
	// will panic because proto.Clone use descriptor dynamically generated by protoparse
	// panic: proto: google.protobuf.Timestamp.seconds: field descriptor does not belong to this message
	// proto.Clone(protomsg)
}
