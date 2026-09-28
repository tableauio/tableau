package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCaptureWritesCPUAndMemoryProfiles(t *testing.T) {
	outputDir := t.TempDir()
	if err := Capture("testgen", outputDir, func() error {
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

	for _, name := range []string{"testgen-cpu.pprof", "testgen-mem.pprof"} {
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
