package xfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDir(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"file in dir", "foo/bar/baz.txt", "foo/bar"},
		{"file in root", "baz.txt", "."},
		{"nested dirs", "a/b/c/d.proto", "a/b/c"},
		// Note: backslash is only a path separator on Windows;
		// on macOS/Linux it is treated as a literal character.
		{"absolute path", "/tmp/foo/bar/baz.txt", "/tmp/foo/bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Dir(tt.path); got != tt.want {
				t.Errorf("Dir(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		name  string
		elems []string
		want  string
	}{
		{"simple join", []string{"foo", "bar", "baz.txt"}, "foo/bar/baz.txt"},
		{"single element", []string{"foo"}, "foo"},
		{"with dot", []string{"foo", ".", "bar"}, "foo/bar"},
		{"with double dot", []string{"foo", "bar", "..", "baz"}, "foo/baz"},
		{"empty elements", []string{"", "foo", "", "bar"}, "foo/bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Join(tt.elems...); got != tt.want {
				t.Errorf("Join(%v) = %q, want %q", tt.elems, got, tt.want)
			}
		})
	}
}

func TestCleanSlashPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"already clean", "foo/bar/baz", "foo/bar/baz"},
		{"trailing slash", "foo/bar/", "foo/bar"},
		{"double slash", "foo//bar", "foo/bar"},
		{"dot segment", "foo/./bar", "foo/bar"},
		{"double dot segment", "foo/bar/../baz", "foo/baz"},
		// Note: backslash is only a path separator on Windows;
		// on macOS/Linux filepath.ToSlash does not convert it.
		{"absolute path", "/tmp/foo/bar", "/tmp/foo/bar"},
		{"empty", "", "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanSlashPath(tt.path); got != tt.want {
				t.Errorf("CleanSlashPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsSamePath(t *testing.T) {
	// Get current working directory to construct absolute paths
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	tests := []struct {
		name      string
		leftPath  string
		rightPath string
		want      bool
	}{
		{
			name:      "identical relative paths",
			leftPath:  "foo/bar/baz.txt",
			rightPath: "foo/bar/baz.txt",
			want:      true,
		},
		{
			name:      "different relative paths",
			leftPath:  "foo/bar/a.txt",
			rightPath: "foo/bar/b.txt",
			want:      false,
		},
		{
			name:      "relative vs absolute same file",
			leftPath:  "foo/bar/baz.txt",
			rightPath: filepath.Join(cwd, "foo/bar/baz.txt"),
			want:      true,
		},
		{
			name:      "relative vs absolute different file",
			leftPath:  "foo/bar/a.txt",
			rightPath: filepath.Join(cwd, "foo/bar/b.txt"),
			want:      false,
		},
		{
			name:      "both absolute same path",
			leftPath:  filepath.Join(cwd, "foo/bar/baz.txt"),
			rightPath: filepath.Join(cwd, "foo/bar/baz.txt"),
			want:      true,
		},
		{
			name:      "both absolute different path",
			leftPath:  filepath.Join(cwd, "foo/bar/a.txt"),
			rightPath: filepath.Join(cwd, "foo/bar/b.txt"),
			want:      false,
		},
		{
			name:      "relative with dot segments vs absolute",
			leftPath:  "foo/bar/../bar/baz.txt",
			rightPath: filepath.Join(cwd, "foo/bar/baz.txt"),
			want:      true,
		},
		{
			name:      "identical absolute paths",
			leftPath:  "/tmp/foo/bar.txt",
			rightPath: "/tmp/foo/bar.txt",
			want:      true,
		},
		{
			name:      "different absolute paths",
			leftPath:  "/tmp/foo/a.txt",
			rightPath: "/tmp/foo/b.txt",
			want:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSamePath(tt.leftPath, tt.rightPath); got != tt.want {
				t.Errorf("IsSamePath(%q, %q) = %v, want %v", tt.leftPath, tt.rightPath, got, tt.want)
			}
		})
	}
}

func TestRel(t *testing.T) {
	tests := []struct {
		name       string
		basepath   string
		targetpath string
		want       string
		wantErr    bool
	}{
		{
			name:       "simple relative",
			basepath:   "foo/bar",
			targetpath: "foo/bar/baz.txt",
			want:       "baz.txt",
			wantErr:    false,
		},
		{
			name:       "go up one level",
			basepath:   "foo/bar/baz",
			targetpath: "foo/bar/other.txt",
			want:       "../other.txt",
			wantErr:    false,
		},
		{
			name:       "same directory",
			basepath:   "foo/bar",
			targetpath: "foo/bar",
			want:       ".",
			wantErr:    false,
		},
		{
			name:       "nested target",
			basepath:   "foo",
			targetpath: "foo/a/b/c.txt",
			want:       "a/b/c.txt",
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Rel(tt.basepath, tt.targetpath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Rel(%q, %q) error = %v, wantErr %v", tt.basepath, tt.targetpath, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Rel(%q, %q) = %q, want %q", tt.basepath, tt.targetpath, got, tt.want)
			}
		})
	}
}
