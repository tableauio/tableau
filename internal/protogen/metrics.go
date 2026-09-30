package protogen

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/profile"
)

func (gen *Generator) run(generate func() error) error {
	if err := gen.measureOperation("prepare_run", func(context.Context) error {
		return gen.prepareRun()
	}); err != nil {
		return err
	}
	execute := func(ctx context.Context) error {
		return gen.measureOperation("generation", func(context.Context) error {
			return generate()
		})
	}
	if !gen.profiling {
		return execute(gen.ctx)
	}
	defer gen.BookMetrics.Print()
	defer gen.SheetParserMetrics.Print()
	profileDir := gen.OutputDir
	if gen.OutputOpt != nil {
		profileDir = filepath.Join(profileDir, gen.OutputOpt.Subdir)
	}
	files, err := profile.Capture("protogen", profileDir, func() error {
		return execute(gen.ctx)
	})
	return errors.Join(err, gen.SheetParserMetrics.LoadCPUProfile(files.CPU), gen.BookMetrics.LoadCPUProfile(files.CPU))
}

func (gen *Generator) measureOperation(name string, operation func(context.Context) error) error {
	return gen.measureOperationWithLabels(name, operation)
}

func (gen *Generator) measureOperationWithLabels(name string, operation func(context.Context) error, labels ...string) error {
	if !gen.profiling {
		return operation(gen.ctx)
	}
	return gen.BookMetrics.Measure(gen.ctx, "protogen", name, operation, labels...)
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
