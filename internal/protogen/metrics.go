package protogen

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/profile"
)

func (gen *Generator) run(work func() error) error {
	run := func(ctx context.Context) error {
		if err := gen.measureWork("prepare_run", func(context.Context) error {
			return gen.prepareRun()
		}); err != nil {
			return err
		}
		return gen.measureWork("generation", func(context.Context) error {
			return work()
		})
	}
	if !gen.profiling {
		return run(gen.ctx)
	}
	defer gen.WorkMetrics.Print()
	defer gen.SheetParserMetrics.Print()
	profileDir := gen.OutputDir
	if gen.OutputOpt != nil {
		profileDir = filepath.Join(profileDir, gen.OutputOpt.Subdir)
	}
	files, err := profile.Capture("protogen", profileDir, func() error {
		return run(gen.ctx)
	})
	return errors.Join(err, gen.SheetParserMetrics.LoadCPUProfile(files.CPU))
}

func (gen *Generator) measureWork(work string, run func(context.Context) error) error {
	return gen.measureWorkWithLabels(work, run)
}

func (gen *Generator) measureWorkWithLabels(work string, run func(context.Context) error, labels ...string) error {
	if !gen.profiling {
		return run(gen.ctx)
	}
	return gen.WorkMetrics.Measure(gen.ctx, "protogen", work, run, labels...)
}

func (gen *Generator) measureSheet(bookName string, pass parsePass, sheet *book.Sheet, parse func() error) error {
	if !gen.profiling {
		return parse()
	}
	key := profile.SheetMetricKey{
		Book:   bookName,
		Sheet:  sheet.GetDebugName(),
		Detail: string(pass),
	}
	return gen.SheetParserMetrics.Measure(gen.ctx, "protogen", key, sheet, func(context.Context) error {
		return parse()
	})
}
