package xproto

import (
	"testing"

	"github.com/jhump/protoreflect/desc"
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
					"proto",          // tableau
				},
				filenames: []string{
					"tableau/protobuf/metabook.proto",
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
