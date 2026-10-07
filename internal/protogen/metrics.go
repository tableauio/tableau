package protogen

import (
	"context"
	"errors"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/profile"
)

// run prepares generation and executes it with optional profiling.
func (gen *Generator) run(generate func(context.Context) error) error {
	if err := gen.measureOperation(gen.ctx, "prepare_run", func(context.Context) error {
		return gen.prepareRun()
	}); err != nil {
		return err
	}
	if !gen.profiling {
		return generate(gen.ctx)
	}
	return gen.runProfiled(generate)
}

func (gen *Generator) runProfiled(generate func(context.Context) error) error {
	defer gen.printMetrics()

	files, runErr := profile.Capture("protogen", gen.output.outputDir, func() error {
		return gen.measureOperation(gen.ctx, "generation", generate)
	})
	sheetErr := gen.SheetParserMetrics.LoadCPUProfile(files.CPU)
	bookErr := gen.BookMetrics.LoadCPUProfile(files.CPU)
	return errors.Join(runErr, sheetErr, bookErr)
}

func (gen *Generator) printMetrics() {
	gen.SheetParserMetrics.Print()
	gen.BookMetrics.Print()
}

func (gen *Generator) measureOperation(ctx context.Context, name string, operation func(context.Context) error, labels ...string) error {
	if !gen.profiling {
		return operation(ctx)
	}
	return gen.BookMetrics.Measure(ctx, "protogen", name, operation, labels...)
}

func (gen *Generator) measureSheet(ctx context.Context, bookName string, pass parsePass, sheet *book.Sheet, parse func() error) error {
	if !gen.profiling {
		return parse()
	}
	key := profile.SheetMetricKey{
		Book:   bookName,
		Sheet:  sheet.GetDebugName(),
		Detail: string(pass),
	}
	return gen.SheetParserMetrics.Measure(ctx, "protogen", key, sheet, func(context.Context) error {
		return parse()
	})
}
