package profile

import (
	"context"
	"runtime/pprof"
)

// Run executes work with the supplied pprof labels and returns its error.
func Run(ctx context.Context, labels pprof.LabelSet, work func(context.Context) error) (err error) {
	if _, enabled := pprof.Label(ctx, "generator"); !enabled {
		return work(ctx)
	}
	pprof.Do(ctx, labels, func(ctx context.Context) {
		err = work(ctx)
	})
	return err
}
