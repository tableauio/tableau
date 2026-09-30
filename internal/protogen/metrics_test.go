package protogen

import (
	"os"
	"path/filepath"
	"runtime/pprof"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/options"
)

func TestProfilingDisabled(t *testing.T) {
	gen := NewGeneratorWithOptions("", t.TempDir(), t.TempDir(), options.NewDefault())
	if value, ok := pprof.Label(gen.ctx, "generator"); ok {
		t.Fatalf("generator label = %q, true; want profiling disabled", value)
	}
}

func TestPrepareRunFailureDoesNotWriteProfiles(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(blocked, []byte("occupied"), 0o600))
	opts := options.NewDefault()
	opts.Profiling = true
	opts.Proto.Output.Subdir = "profiles"
	gen := NewGeneratorWithOptions("", t.TempDir(), blocked, opts)
	require.Error(t, gen.Generate())
	require.NoDirExists(t, filepath.Join(blocked, "profiles"))
	require.NoFileExists(t, filepath.Join(blocked, "profiles", "protogen-cpu.pprof"))
	require.NoFileExists(t, filepath.Join(blocked, "profiles", "protogen-mem.pprof"))
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

	for _, name := range []string{"protogen-cpu.pprof", "protogen-mem.pprof"} {
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
