package profile

import (
	"context"
	"errors"
	"runtime/pprof"
	"testing"
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
