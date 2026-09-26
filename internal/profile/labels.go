package profile

import (
	"context"
	"runtime/pprof"
)

// WithGenerator records the profiled generator name in ctx.
func WithGenerator(ctx context.Context, name string) context.Context {
	return pprof.WithLabels(ctx, pprof.Labels("generator", name))
}

// Run executes profiled work with alternating pprof label keys and values.
// Callers invoke Run only while profiling is enabled.
func Run(ctx context.Context, work func(context.Context) error, labels ...string) (err error) {
	pprof.Do(ctx, pprof.Labels(labels...), func(ctx context.Context) {
		err = work(ctx)
	})
	return err
}
