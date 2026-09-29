package protogen

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/profile"
)

func (gen *Generator) run(work func() error) error {
	if err := gen.prepareRun(); err != nil {
		return err
	}
	if !gen.profiling {
		return work()
	}
	defer gen.SheetParserMetrics.Print()
	profileDir := gen.OutputDir
	if gen.OutputOpt != nil {
		profileDir = filepath.Join(profileDir, gen.OutputOpt.Subdir)
	}
	files, err := profile.Capture("protogen", profileDir, work)
	return errors.Join(err, gen.SheetParserMetrics.LoadCPUProfile(files.CPU))
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
