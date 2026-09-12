package protoc

import (
	"os"
	"path/filepath"
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

func Test_rel(t *testing.T) {
	tests := []struct {
		name       string
		filename   string
		protoPaths []string
		want       string
		wantErr    bool
	}{
		{
			name:     "sibling protoPaths prefer containing dir not parent-relative",
			filename: "Temp/proto/generated/common_enum_conf.proto",
			protoPaths: []string{
				"Temp/proto/common",
				"Temp/proto/generated",
			},
			want: "common_enum_conf.proto",
		},
		{
			name:     "most specific protoPath wins",
			filename: "Temp/proto/generated/common_enum_conf.proto",
			protoPaths: []string{
				"Temp/proto",
				"Temp/proto/generated",
			},
			want: "common_enum_conf.proto",
		},
		{
			name:     "nested under first protoPath",
			filename: "proto/tableau/protobuf/unittest/common.proto",
			protoPaths: []string{
				"proto",
			},
			want: "tableau/protobuf/unittest/common.proto",
		},
		{
			name:     "no containing protoPath is an error",
			filename: "Temp/proto/generated/foo.proto",
			protoPaths: []string{
				"Temp/proto/common",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rel(tt.filename, tt.protoPaths)
			if (err != nil) != tt.wantErr {
				t.Errorf("rel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("rel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewFiles_siblingProtoPathsNoDuplicateSymbols(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	generatedDir := filepath.Join(root, "generated")
	if err := os.MkdirAll(commonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(generatedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	commonProto := `syntax = "proto3";
package protoconf;
message Item { int32 id = 1; }
`
	enumProto := `syntax = "proto3";
package protoconf;
enum AttributeId { ATTRIBUTE_ID_UNSPECIFIED = 0; }
`
	bookProto := `syntax = "proto3";
package protoconf;
import "common.proto";
import "enum.proto";
message Book {
  Item item = 1;
  AttributeId attr = 2;
}
`
	if err := os.WriteFile(filepath.Join(commonDir, "common.proto"), []byte(commonProto), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(generatedDir, "enum.proto"), []byte(enumProto), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(generatedDir, "book.proto"), []byte(bookProto), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := NewFiles(
		[]string{commonDir, generatedDir},
		[]string{
			filepath.Join(commonDir, "common.proto"),
			filepath.Join(generatedDir, "*.proto"),
		},
	)
	if err != nil {
		t.Fatalf("NewFiles() unexpected error: %v", err)
	}
	for _, path := range []string{"common.proto", "enum.proto", "book.proto"} {
		if _, err := files.FindFileByPath(path); err != nil {
			t.Errorf("FindFileByPath(%q) = %v", path, err)
		}
	}
	if _, err := files.FindFileByPath("../generated/enum.proto"); err == nil {
		t.Error("FindFileByPath(../generated/enum.proto) succeeded; file should be registered as enum.proto")
	}
}

func TestNewFiles_protoFileOutsideProtoPaths(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	otherDir := filepath.Join(root, "other")
	if err := os.MkdirAll(commonDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	proto := `syntax = "proto3";
package protoconf;
message Item { int32 id = 1; }
`
	path := filepath.Join(otherDir, "item.proto")
	if err := os.WriteFile(path, []byte(proto), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := NewFiles([]string{commonDir}, []string{path})
	if err == nil {
		t.Fatal("NewFiles() expected error when proto file is not under any protoPath")
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
