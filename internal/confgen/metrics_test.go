package confgen

import (
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/tableauio/tableau/options"
)

func TestProfilingDisabled(t *testing.T) {
	gen := NewGeneratorWithOptions("", "", t.TempDir(), options.NewDefault())
	if value, ok := pprof.Label(gen.ctx, "generator"); ok {
		t.Fatalf("generator label = %q, true; want profiling disabled", value)
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

	if err := gen.runProfiling(func() error {
		data := make([]byte, 1<<20)
		deadline := time.Now().Add(100 * time.Millisecond)
		for time.Now().Before(deadline) {
			data[0]++
		}
		runtime.KeepAlive(data)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"confgen-cpu.pprof", "confgen-mem.pprof"} {
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
