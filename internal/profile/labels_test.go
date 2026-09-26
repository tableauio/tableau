package profile

import (
	"context"
	"errors"
	"runtime/pprof"
	"testing"
)

func TestRunWithoutProfiling(t *testing.T) {
	base := context.Background()
	ctx := WithGenerator(base, "test", false)
	if ctx != base {
		t.Fatal("WithGenerator() changed the context while profiling is disabled")
	}
	if Enabled(ctx) {
		t.Fatal("Enabled() = true while profiling is disabled")
	}

	wantErr := errors.New("work failed")
	err := Run(ctx, func(ctx context.Context) error {
		if _, ok := pprof.Label(ctx, "work"); ok {
			t.Fatal("profiling label added while profiling is disabled")
		}
		return wantErr
	}, "work", "load")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestRunWithoutProfilingDoesNotAllocate(t *testing.T) {
	ctx := context.Background()
	work := func(context.Context) error { return nil }
	allocs := testing.AllocsPerRun(1000, func() {
		_ = Run(ctx, work, "work", "load")
	})
	if allocs != 0 {
		t.Fatalf("Run() allocations = %v, want 0 when profiling is disabled", allocs)
	}
}

func TestRunWithProfiling(t *testing.T) {
	ctx := WithGenerator(context.Background(), "test", true)
	if !Enabled(ctx) {
		t.Fatal("Enabled() = false while profiling is enabled")
	}
	err := Run(ctx, func(ctx context.Context) error {
		if value, ok := pprof.Label(ctx, "work"); !ok || value != "load" {
			t.Fatalf("work label = %q, %v; want load, true", value, ok)
		}
		return nil
	}, "work", "load")
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkRunWithoutProfiling(b *testing.B) {
	ctx := WithGenerator(context.Background(), "test", false)
	work := func(context.Context) error { return nil }
	b.ReportAllocs()
	for b.Loop() {
		_ = Run(ctx, work, "work", "load")
	}
}
