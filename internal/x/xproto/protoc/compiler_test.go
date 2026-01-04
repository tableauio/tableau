package protoc

import (
	"testing"
)

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
					"../../../../proto", // tableau
				},
				protoFiles: []string{
					"../../../../proto/tableau/protobuf/unittest/*.proto",
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

func TestParseProtos(t *testing.T) {
	type args struct {
		protoPaths []string
		protoFiles   []string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
		{
			name: "test1",
			args: args{
				protoPaths: []string{
					"../../../../proto", // tableau
				},
				protoFiles: []string{
					"tableau/protobuf/unittest/unittest.proto",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := ParseProtos(tt.args.protoPaths, tt.args.protoFiles...)
			if err != nil {
				t.Errorf("parseProtos() error = %v", err)
			}
			t.Logf("parsed proto files: %+v", files)
		})
	}
}

