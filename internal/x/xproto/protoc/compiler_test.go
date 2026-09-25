package protoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeProtoFile(t *testing.T, path, source string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewFilesExcludesSelectedProto(t *testing.T) {
	root := "../../../../proto"
	files, err := NewFiles(
		[]string{root},
		[]string{filepath.Join(root, "tableau/protobuf/unittest/*.proto")},
		filepath.Join(root, "tableau/protobuf/unittest/unittest.proto"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := files.FindFileByPath("tableau/protobuf/unittest/common.proto"); err != nil {
		t.Fatalf("selected proto is missing: %v", err)
	}
	if _, err := files.FindFileByPath("tableau/protobuf/unittest/unittest.proto"); err == nil {
		t.Fatal("excluded proto was compiled")
	}
}

func TestRel(t *testing.T) {
	tests := []struct {
		name       string
		filename   string
		protoPaths []string
		want       string
		wantErr    bool
	}{
		{
			name:       "use configured root for sibling directory",
			filename:   "proto/generated/item.proto",
			protoPaths: []string{"proto/common", "proto/generated"},
			want:       "item.proto",
		},
		{
			name:       "first containing root defines import path",
			filename:   "proto/generated/item.proto",
			protoPaths: []string{"proto", "proto/generated"},
			want:       "generated/item.proto",
		},
		{
			name:       "file outside roots",
			filename:   "proto/generated/item.proto",
			protoPaths: []string{"proto/common"},
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := rel(tt.filename, tt.protoPaths)
			if (err != nil) != tt.wantErr {
				t.Fatalf("rel() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("rel() = %q, want %q", got, tt.want)
			}
		})
	}
}
func TestNewFilesResolvesImportsAcrossSiblingRoots(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	generatedDir := filepath.Join(root, "generated")
	commonPath := filepath.Join(commonDir, "common.proto")
	bookPath := filepath.Join(generatedDir, "book.proto")
	writeProtoFile(t, commonPath, `syntax = "proto3";
package protoconf;
message Common {}
`)
	writeProtoFile(t, bookPath, `syntax = "proto3";
package protoconf;
import "common.proto";
message Book { Common common = 1; }
`)

	files, err := NewFiles([]string{commonDir, generatedDir}, []string{commonPath, bookPath})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"common.proto", "book.proto"} {
		if _, err := files.FindFileByPath(path); err != nil {
			t.Errorf("compiled proto %q is missing: %v", path, err)
		}
	}
}

func TestNewFilesUsesFirstRootForNestedImports(t *testing.T) {
	root := t.TempDir()
	commonDir := filepath.Join(root, "common")
	commonPath := filepath.Join(commonDir, "common.proto")
	bookPath := filepath.Join(root, "book.proto")
	writeProtoFile(t, commonPath, `syntax = "proto3";
package protoconf;
message Common {}
`)
	writeProtoFile(t, bookPath, `syntax = "proto3";
package protoconf;
import "common/common.proto";
message Book { Common common = 1; }
`)

	files, err := NewFiles([]string{root, commonDir}, []string{commonPath, bookPath})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := files.FindFileByPath("common/common.proto"); err != nil {
		t.Fatalf("imported proto is missing: %v", err)
	}
	if _, err := files.FindFileByPath("common.proto"); err == nil {
		t.Fatal("imported proto was also compiled under the nested root name")
	}
}

func TestNewFilesRejectsFileOutsideProtoPaths(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "other", "item.proto")
	writeProtoFile(t, path, `syntax = "proto3";
package protoconf;
message Item {}
`)

	_, err := NewFiles([]string{filepath.Join(root, "common")}, []string{path})
	if err == nil || !strings.Contains(err.Error(), "not under any protoPath") {
		t.Fatalf("NewFiles() error = %v, want file outside proto paths", err)
	}
}

func TestNewFilesRejectsDuplicateImportPaths(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "common", "shared.proto")
	second := filepath.Join(root, "generated", "shared.proto")
	const source = `syntax = "proto3";
package protoconf;
`
	writeProtoFile(t, first, source)
	writeProtoFile(t, second, source)

	_, err := NewFiles(
		[]string{filepath.Dir(first), filepath.Dir(second)},
		[]string{first, second},
	)
	if err == nil {
		t.Fatal("expected an error for two selected files named shared.proto")
	}
	for _, path := range []string{"shared.proto", filepath.ToSlash(first), filepath.ToSlash(second)} {
		if !strings.Contains(err.Error(), path) {
			t.Errorf("error %q does not identify %q", err, path)
		}
	}
}

func TestNewFilesPredefinedProtoCannotImportGeneratedProto(t *testing.T) {
	root := t.TempDir()
	predefined := filepath.Join(root, "predefined.proto")
	generated := filepath.Join(root, "generated.proto")
	writeProtoFile(t, predefined, `syntax = "proto3";
package protoconf;
import "generated.proto";
message Predefined { Generated value = 1; }
`)
	writeProtoFile(t, generated, `syntax = "proto3";
package protoconf;
message Generated {}
`)

	_, err := NewFiles([]string{root}, []string{predefined})
	if err == nil || !strings.Contains(err.Error(), "generated.proto") {
		t.Fatalf("NewFiles() error = %v, want unresolved generated.proto import", err)
	}
}
