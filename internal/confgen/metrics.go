package confgen

import (
	"path/filepath"

	"github.com/tableauio/tableau/internal/profile"
	"github.com/tableauio/tableau/log"
)

func (gen *Generator) startProfiling() (func() error, error) {
	profileDir := gen.OutputDir
	if gen.OutputOpt != nil {
		profileDir = filepath.Join(profileDir, gen.OutputOpt.Subdir)
	}
	return profile.Start("confgen", profileDir)
}

// printMetrics reports importer cache use and sheet parser metrics.
func (gen *Generator) printMetrics() {
	requests, imports, sheets, paths := gen.importerCache.Metrics()
	log.Infof("importer cache: requests=%d imports=%d sheets=%d paths=%d", requests, imports, sheets, paths)
	gen.SheetParserMetrics.Print()
}
