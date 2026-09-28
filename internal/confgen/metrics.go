package confgen

import (
	"path/filepath"

	"github.com/tableauio/tableau/internal/profile"
	"github.com/tableauio/tableau/log"
)

func (gen *Generator) runProfiling(work func() error) error {
	if !gen.profiling {
		return work()
	}
	defer gen.printMetrics()
	profileDir := gen.OutputDir
	if gen.OutputOpt != nil {
		profileDir = filepath.Join(profileDir, gen.OutputOpt.Subdir)
	}
	return profile.Capture("confgen", profileDir, work)
}

// printMetrics reports importer cache use and sheet parser metrics.
func (gen *Generator) printMetrics() {
	requests, imports, sheets, paths := gen.importerCache.Metrics()
	log.Infof("importer cache: requests=%d imports=%d sheets=%d paths=%d", requests, imports, sheets, paths)
	gen.SheetParserMetrics.Print()
}
