package xproto

import (
	"testing"

	"github.com/jhump/protoreflect/desc"
	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/internal/types"
	_ "github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func Test_ParseProtos(t *testing.T) {
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
					"proto", // tableau
				},
				filenames: []string{
					"tableau/protobuf/unittest.proto",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseProtos(tt.args.ImportPaths, tt.args.filenames...)
			if err != nil {
				t.Errorf("parseProtos() error = %v", err)
			}
		})
	}
}

func Test_extractTypeInfosFromMessage(t *testing.T) {
	desc1, err := protoregistry.GlobalFiles.FindDescriptorByName("tableau.TestItem")
	if err != nil {
		t.Fatalf("descriptor not found")
	}
	md1 := desc1.(protoreflect.MessageDescriptor)
	desc2, err := protoregistry.GlobalFiles.FindDescriptorByName("tableau.TestTarget")
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
				protoPackage: "tableau",
				infos: map[string]*TypeInfo{
					"tableau.TestItem": {
						FullName:       "tableau.TestItem",
						ParentFilename: "tableau/protobuf/unittest.proto",
						Kind:           types.MessageKind,
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
				protoPackage: "tableau",
				infos: map[string]*TypeInfo{
					"tableau.TestTarget": {
						FullName:       "tableau.TestTarget",
						ParentFilename: "tableau/protobuf/unittest.proto",
						Kind:           types.MessageKind,
					},

					"tableau.TestTarget.Type": {
						FullName:       "tableau.TestTarget.Type",
						ParentFilename: "tableau/protobuf/unittest.proto",
						Kind:           types.EnumKind,
					},
					"tableau.TestTarget.Pvp": {
						FullName:       "tableau.TestTarget.Pvp",
						ParentFilename: "tableau/protobuf/unittest.proto",
						Kind:           types.MessageKind,
					},
					"tableau.TestTarget.Pve": {
						FullName:       "tableau.TestTarget.Pve",
						ParentFilename: "tableau/protobuf/unittest.proto",
						Kind:           types.MessageKind,
					},
					"tableau.TestTarget.Pve.Mission": {
						FullName:       "tableau.TestTarget.Pve.Mission",
						ParentFilename: "tableau/protobuf/unittest.proto",
						Kind:           types.MessageKind,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewTypeInfos("tableau")
			extractTypeInfosFromMessage(tt.args.md, got)
			assert.Equal(t, tt.want, got, "extractTypeInfosFromMessage")
		})
	}
}
