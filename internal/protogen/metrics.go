package protogen

import (
	"path/filepath"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/profile"
)

func (gen *Generator) startProfiling() (func() error, error) {
	if !gen.profiling {
		return func() error { return nil }, nil
	}

	profileDir := gen.OutputDir
	if gen.OutputOpt != nil {
		profileDir = filepath.Join(profileDir, gen.OutputOpt.Subdir)
	}
	return profile.Start("protogen", profileDir)
}

func (gen *Generator) measureSheet(bookName, detail string, sheet *book.Sheet, parse func() error) error {
	if !gen.profiling {
		return parse()
	}
	key := profile.SheetMetricKey{
		Book:   bookName,
		Sheet:  sheet.GetDebugName(),
		Detail: detail,
	}
	return gen.SheetParserMetrics.Measure("protogen_sheet", key, sheet, parse)
}

// PrintSheetMetrics reports sheet parser metrics for a protogen run.
func PrintSheetMetrics(gen *Generator) {
	gen.SheetParserMetrics.Print()
}

func formatParsePass(pass parsePass) string {
	if pass == firstPass {
		return "first-pass"
	}
	return "second-pass"
}
