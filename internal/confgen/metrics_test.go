package confgen

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

func TestProfilingUsesRuntimeOption(t *testing.T) {
	outputDir := t.TempDir()
	opts := options.NewDefault()
	opts.Profiling = true
	opts.Conf.Output.Subdir = "profiles"
	gen := NewGeneratorWithOptions("", "", outputDir, opts)
	if value, ok := pprof.Label(gen.ctx, "generator"); !ok || value != "confgen" {
		t.Fatalf("generator label = %q, %t; want confgen, true", value, ok)
	}

	stop, err := gen.startProfiling()
	if err != nil {
		t.Fatal(err)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"confgen-cpu.pprof", "confgen-mem.pprof", "confgen-block.pprof"} {
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
