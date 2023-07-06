package xproto

import (
	"reflect"
	"testing"

	"github.com/jhump/protoreflect/desc"
	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/internal/types"
	_ "github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func TestParseProtos(t *testing.T) {
	type args struct {
		ImportPaths []string
		filenames   []string
	}
	tests := []struct {
		name    string
		args    args
		want    []*desc.FileDescriptor
		wantErr bool
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
