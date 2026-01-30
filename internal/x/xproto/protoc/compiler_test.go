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
		name                  string
		args                  args
		wantErr               bool
		wantProtoFiles        []string
		wantExcludeProtoFiles []string
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
				excludeProtoFiles: []string{
					"../../../../proto/tableau/protobuf/unittest/unittest.proto",
				},
			},
			wantErr: false,
			wantProtoFiles: []string{
				"tableau/protobuf/unittest/common.proto",
			},
			wantExcludeProtoFiles: []string{
				"tableau/protobuf/unittest/unittest.proto",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := NewFiles(tt.args.protoPaths, tt.args.protoFiles, tt.args.excludeProtoFiles...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for _, file := range tt.wantProtoFiles {
				if _, err := files.FindFileByPath(file); err != nil {
					t.Errorf("NewFiles() wantProtoFile %v not found", file)
					return
				}
			}
			for _, file := range tt.wantExcludeProtoFiles {
				if _, err := files.FindFileByPath(file); err == nil {
					t.Errorf("NewFiles() wantExcludeProtoFile %v found", file)
					return
				}
			}
		})
	}
}

func Test_parseProtos(t *testing.T) {
	type args struct {
		protoPaths []string
		protoFiles map[string]string
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
				protoFiles: map[string]string{
					"../../../../proto/tableau/protobuf/unittest/unittest.proto": "tableau/protobuf/unittest/unittest.proto",
					"../../../../proto/tableau/protobuf/unittest/common.proto":   "tableau/protobuf/unittest/common.proto",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := parseProtos(tt.args.protoPaths, tt.args.protoFiles)
			if err != nil {
				t.Errorf("parseProtos() error = %v", err)
				return
			}
			t.Logf("parsed proto files: %+v", files)
		})
	}
}
