package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestStartWritesCPUAndMemoryProfiles(t *testing.T) {
	outputDir := t.TempDir()
	stop, err := Start("testgen", outputDir)
	if err != nil {
		t.Fatal(err)
	}

	data := make([]byte, 1<<20)
	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		data[0]++
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	runtime.KeepAlive(data)

	for _, name := range []string{"testgen-cpu.pprof", "testgen-mem.pprof", "testgen-block.pprof"} {
		info, err := os.Stat(filepath.Join(outputDir, name))
		if err != nil {
			t.Errorf("stat profile %s: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("profile %s is empty", name)
		}
	}
}
