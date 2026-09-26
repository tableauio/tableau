package profile

import (
	"context"
	"runtime/pprof"
)

// WithGenerator marks ctx as profiling-enabled and records the generator name.
// It returns ctx unchanged when enabled is false.
func WithGenerator(ctx context.Context, name string, enabled bool) context.Context {
	if !enabled {
		return ctx
	}
	return pprof.WithLabels(ctx, pprof.Labels("generator", name))
}

// Enabled reports whether ctx belongs to a profiling-enabled generator.
func Enabled(ctx context.Context) bool {
	_, enabled := pprof.Label(ctx, "generator")
	return enabled
}

// Run executes work with alternating pprof label keys and values. When the
// generator is not being profiled, it runs work directly without constructing
// a label set or changing the context.
func Run(ctx context.Context, work func(context.Context) error, labels ...string) (err error) {
	if !Enabled(ctx) {
		return work(ctx)
	}
	pprof.Do(ctx, pprof.Labels(labels...), func(ctx context.Context) {
		err = work(ctx)
	})
	return err
}
