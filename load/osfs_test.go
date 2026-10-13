package load

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestWithReadFunc_setsFS(t *testing.T) {
	opts := ParseOptions(WithReadFunc(func(name string) ([]byte, error) {
		return []byte("from-readfunc:" + name), nil
	}))
	got, err := fs.ReadFile(opts.ParseMessagerOptionsByName("Any").GetFS(), "any.json")
	if err != nil {
		t.Fatal(err)
	}
	// ReadFunc receives OS-native path (FromSlash applied in adapter).
	if string(got) != "from-readfunc:any.json" {
		t.Fatalf("got %q", got)
	}
}

func TestWithReadFunc_andWithFileSystem_lastWins(t *testing.T) {
	mapFS := fstest.MapFS{"x.json": {Data: []byte("map")}}
	read := ReadFunc(func(string) ([]byte, error) { return []byte("read"), nil })

	// FileSystem after ReadFunc → MapFS wins
	opts := ParseOptions(WithReadFunc(read), WithFileSystem(mapFS))
	got, err := fs.ReadFile(opts.ParseMessagerOptionsByName("X").GetFS(), "x.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "map" {
		t.Fatalf("FileSystem should win when set last: got %q", got)
	}

	// ReadFunc after FileSystem → ReadFunc wrapper wins
	opts = ParseOptions(WithFileSystem(mapFS), WithReadFunc(read))
	got, err = fs.ReadFile(opts.ParseMessagerOptionsByName("X").GetFS(), "x.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "read" {
		t.Fatalf("ReadFunc should win when set last: got %q", got)
	}
}

func TestReadFuncFS_globUsesHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.json")
	if err := os.WriteFile(path, []byte(`{"raw":1}`), 0o644); err != nil {
		t.Fatal(err)
	}

	fsys := readFuncFS{read: func(name string) ([]byte, error) {
		data, err := os.ReadFile(name)
		if err != nil {
			return nil, err
		}
		return append([]byte("wrapped:"), data...), nil
	}}

	matches, err := fs.Glob(fsys, filepath.ToSlash(filepath.Join(dir, "*.json")))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("glob matches: %v", matches)
	}

	got, err := fs.ReadFile(fsys, filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "wrapped:{\"raw\":1}" {
		t.Fatalf("got %q", got)
	}
}
