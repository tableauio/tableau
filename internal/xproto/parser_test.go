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
					"../../proto",          // tableau
					"../../test/dev/proto", // protoconf
				},
				filenames: []string{
					"common.proto",
					// "time.proto",
					// "cs_dbkeyword.proto",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseProtos(tt.args.ImportPaths, tt.args.filenames...)
			if err != nil {
				t.Errorf("parseProtos() error = %v", err)
			}
			t.Errorf("parseProtos() got = %v", got)
			// if (err != nil) != tt.wantErr {
			// 	t.Errorf("parseProtos() error = %v, wantErr %v", err, tt.wantErr)
			// 	return
			// }
			// if !reflect.DeepEqual(got, tt.want) {
			// 	t.Errorf("parseProtos() = %v, want %v", got, tt.want)
			// }
		})
	}
}
