package protogen

import (
	"context"
	"path/filepath"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/profile"
)

func (gen *Generator) runProfiling(work func() error) error {
	if !gen.profiling {
		return work()
	}
	defer gen.SheetParserMetrics.Print()
	profileDir := gen.OutputDir
	if gen.OutputOpt != nil {
		profileDir = filepath.Join(profileDir, gen.OutputOpt.Subdir)
	}
	return profile.Capture("protogen", profileDir, work)
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
