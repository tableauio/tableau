package profile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime/pprof"
	"testing"
	"time"

	profiledata "github.com/google/pprof/profile"
)

func TestBookMetricsMeasure(t *testing.T) {
	var metrics BookMetrics
	wantErr := errors.New("failed")
	err := metrics.Measure(context.Background(), "protogen", "import_xlsx", func(ctx context.Context) error {
		for key, want := range map[string]string{
			"generator": "protogen",
			"name":      "import_xlsx",
			"book":      "Item.xlsx",
		} {
			if got, ok := pprof.Label(ctx, key); !ok || got != want {
				t.Errorf("label %s = %q, %t; want %q, true", key, got, ok, want)
			}
		}
		return wantErr
	}, "book", "Item.xlsx")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Measure() error = %v, want %v", err, wantErr)
	}

	snapshot := metrics.snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("metric count = %d, want 1", len(snapshot))
	}
	got := snapshot[0]
	if got.name != "import_xlsx" || got.calls != 1 || got.failures != 1 {
		t.Fatalf("metric = %+v", got)
	}
	if got.totalWallTime < 0 || got.maxWallTime < 0 || got.maxWallTime > got.totalWallTime {
		t.Fatalf("metric timing = %+v", got)
	}
}

func TestBookMetricsCPUAndReset(t *testing.T) {
	var metrics BookMetrics
	metrics.record("import_xlsx", time.Second, false)
	filename := filepath.Join(t.TempDir(), "cpu.pprof")
	file, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	cpu := &profiledata.Profile{
		SampleType: []*profiledata.ValueType{{Type: "cpu", Unit: "nanoseconds"}},
		Sample: []*profiledata.Sample{{
			Value: []int64{250_000_000},
			Label: map[string][]string{"name": {"import_xlsx"}},
		}},
	}
	if err := cpu.Write(file); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := metrics.LoadCPUProfile(filename); err != nil {
		t.Fatal(err)
	}
	if got := metrics.snapshot()[0].cpuTime; got != 250*time.Millisecond {
		t.Fatalf("CPU time = %s, want 250ms", got)
	}
	metrics.Reset()
	if got := metrics.snapshot(); len(got) != 0 {
		t.Fatalf("metrics after Reset() = %+v", got)
	}
}
