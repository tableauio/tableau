package confgen

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/tableauio/tableau/internal/profile"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/log"
)

// run completes generation and profiling before returning structured errors.
func (gen *Generator) run(work func() error) error {
	if !gen.profiling {
		return xerrors.Normalize(work())
	}
	defer gen.printMetrics()
	profileDir := gen.OutputDir
	if gen.OutputOpt != nil {
		profileDir = filepath.Join(profileDir, gen.OutputOpt.Subdir)
	}
	files, err := profile.Capture("confgen", profileDir, work)
	return xerrors.Normalize(errors.Join(err, gen.SheetParserMetrics.LoadCPUProfile(files.CPU)))
}

// printMetrics reports importer cache use and sheet parser metrics.
func (gen *Generator) printMetrics() {
	requests, imports, sheets, paths := gen.importerCache.Metrics()
	log.Info(formatImporterCacheMetrics(requests, imports, sheets, paths))
	gen.SheetParserMetrics.Print()
}

func formatImporterCacheMetrics(requests, imports, sheets, paths int64) string {
	var output strings.Builder
	output.WriteString("importer cache metrics:\n")
	w := tabwriter.NewWriter(&output, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "METRIC\tVALUE")
	_, _ = fmt.Fprintf(w, "Load requests\t%d\n", requests)
	_, _ = fmt.Fprintf(w, "Opened sources\t%d\n", imports)
	_, _ = fmt.Fprintf(w, "Decoded sheets\t%d\n", sheets)
	_, _ = fmt.Fprintf(w, "Unique paths\t%d\n", paths)
	_ = w.Flush()
	return strings.TrimRight(output.String(), "\n")
}
