package protoc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewFiles_UsesMostSpecificProtoPathForOverlappingRoots(t *testing.T) {
	root := t.TempDir()
	generatedDir := filepath.Join(root, "Temp", "proto")
	if err := os.MkdirAll(generatedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	common := filepath.Join(generatedDir, "common_enum_conf.proto")
	if err := os.WriteFile(common, []byte(`syntax = "proto3";
package protoconf;
enum AttributeId {
  ATTRIBUTE_ID_UNSPECIFIED = 0;
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	activity := filepath.Join(generatedDir, "activity_target_conf.proto")
	if err := os.WriteFile(activity, []byte(`syntax = "proto3";
package protoconf;
import "common_enum_conf.proto";
message ActivityTarget {
  AttributeId attribute_id = 1;
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := NewFiles(
		[]string{root, generatedDir},
		[]string{filepath.Join(generatedDir, "*.proto")},
	)
	if err != nil {
		t.Fatalf("NewFiles() error = %v", err)
	}
	if _, err := files.FindFileByPath("common_enum_conf.proto"); err != nil {
		t.Fatalf("common_enum_conf.proto was not registered by its import path: %v", err)
	}
}

func TestRel_PreservesRelativeFallbackOutsideProtoRoots(t *testing.T) {
	root := filepath.Join("root", "proto")
	filename := filepath.Join("shared", "types.proto")

	got := rel(filename, []string{root})
	want, err := filepath.Rel(root, filename)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.ToSlash(want) {
		t.Fatalf("rel() = %q, want %q", got, filepath.ToSlash(want))
	}
}
