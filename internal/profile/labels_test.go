package profile

import (
	"context"
	"errors"
	"runtime/pprof"
	"testing"
)

func TestRun(t *testing.T) {
	ctx := WithGenerator(context.Background(), "test")
	wantErr := errors.New("work failed")
	err := Run(ctx, func(ctx context.Context) error {
		if value, ok := pprof.Label(ctx, "work"); !ok || value != "load" {
			t.Fatalf("work label = %q, %v; want load, true", value, ok)
		}
		return wantErr
	}, "work", "load")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}
