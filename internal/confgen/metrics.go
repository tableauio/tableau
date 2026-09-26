package confgen

import (
	"path/filepath"

	"github.com/tableauio/tableau/internal/profile"
	"github.com/tableauio/tableau/log"
)

func (gen *Generator) startProfiling() (func() error, error) {
	if !gen.profiling {
		return func() error { return nil }, nil
	}

	profileDir := gen.OutputDir
	if gen.OutputOpt != nil {
		profileDir = filepath.Join(profileDir, gen.OutputOpt.Subdir)
	}
	return profile.Start("confgen", profileDir)
}

// PrintSheetMetrics reports importer cache use and sheet parser metrics.
func PrintSheetMetrics(gen *Generator) {
	if gen.profiling {
		requests, imports, sheets, paths := gen.importerCache.Metrics()
		log.Infof("importer cache: requests=%d imports=%d sheets=%d paths=%d", requests, imports, sheets, paths)
	}
	gen.SheetParserMetrics.Print()
}
