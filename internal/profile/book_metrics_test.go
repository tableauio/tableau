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
			"work":      "import_xlsx",
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

	results := metrics.collect()
	if len(results) != 1 {
		t.Fatalf("metric count = %d, want 1", len(results))
	}
	got := results[0]
	if got.work != "import_xlsx" || got.calls != 1 || got.failures != 1 {
		t.Fatalf("metric = %+v", got)
	}
	if got.wallTime < 0 || got.maxTime < 0 || got.maxTime > got.wallTime {
		t.Fatalf("metric timing = %+v", got)
	}
}
