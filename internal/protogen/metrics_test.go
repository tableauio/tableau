package protogen

import (
	"os"
	"path/filepath"
	"runtime/pprof"
	"testing"

	"github.com/tableauio/tableau/options"
)

func TestProfilingDisabled(t *testing.T) {
	gen := &Generator{OutputDir: t.TempDir()}

	stop, err := gen.startProfiling()
	if err != nil {
		t.Fatal(err)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(gen.OutputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("disabled profiling wrote %d files", len(entries))
	}
}

func TestGenerateWritesProfiles(t *testing.T) {
	inputDir := t.TempDir()
	outputDir := t.TempDir()
	opts := options.NewDefault()
	opts.Profiling = true
	opts.Proto.Output.Subdir = "profiles"
	gen := NewGeneratorWithOptions("", inputDir, outputDir, opts)
	if value, ok := pprof.Label(gen.ctx, "generator"); !ok || value != "protogen" {
		t.Fatalf("generator label = %q, %t; want protogen, true", value, ok)
	}

	if err := gen.Generate(); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"protogen-cpu.pprof", "protogen-mem.pprof", "protogen-block.pprof"} {
		info, err := os.Stat(filepath.Join(outputDir, "profiles", name))
		if err != nil {
			t.Errorf("stat profile %s: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("profile %s is empty", name)
		}
	}
}
