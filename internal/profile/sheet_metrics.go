package profile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	profiledata "github.com/google/pprof/profile"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/log"
)

// SheetMetricKey identifies one generator operation on an imported sheet.
type SheetMetricKey struct {
	Book   string // Book is the source workbook name relative to the input directory.
	Sheet  string // Sheet is the imported worksheet name.
	Detail string // Detail distinguishes operations such as a message or parsing pass.
}

func (k SheetMetricKey) String() string {
	name := k.Book + "#" + k.Sheet
	if k.Detail == "" {
		return name
	}
	return name + " (" + k.Detail + ")"
}

// sheetKind identifies which input metrics apply to a sheet.
type sheetKind string

const (
	tableKind    sheetKind = "table"
	documentKind sheetKind = "document"
)

// sheetInputMetrics describes the source data observed by one parser call.
// Table and document sheets populate only their applicable fields.
type sheetInputMetrics struct {
	kind         sheetKind // kind identifies the table or document representation.
	rows         int64     // rows counts all table rows, including header rows.
	cols         int64     // cols is the widest table row.
	presentCells int64     // presentCells counts explicitly stored, nonempty table cells.
	emptyCells   int64     // emptyCells counts explicitly stored empty table cells.
	missingCells int64     // missingCells counts implicit trailing cells in short table rows.
	emptyRows    int64     // emptyRows counts table rows with no nonempty cells.
	valueBytes   int64     // valueBytes totals bytes in table cells or document scalar values.
	nodes        int64     // nodes counts all document nodes, including the root.
	scalarNodes  int64     // scalarNodes counts document scalar nodes, including empty values.
	maxDepth     int64     // maxDepth is the document depth with the root at depth one.
}

// measureSheetInput measures the imported sheet before parsing can add virtual
// nodes. Missing table cells are the implicit cells after short rows up to the
// widest row. Empty cells are explicitly stored empty strings.
func measureSheetInput(sheet *book.Sheet) sheetInputMetrics {
	var metrics sheetInputMetrics
	if sheet.Document != nil {
		metrics.kind = documentKind
		var visit func(*book.Node, int64)
		visit = func(node *book.Node, depth int64) {
			if node == nil {
				return
			}
			metrics.nodes++
			metrics.maxDepth = max(metrics.maxDepth, depth)
			if node.Kind == book.ScalarNode {
				metrics.scalarNodes++
				metrics.valueBytes += int64(len(node.Value))
			}
			for _, child := range node.Children {
				visit(child, depth+1)
			}
		}
		visit(sheet.Document, 1)
		return metrics
	}
	if sheet.Table == nil {
		return metrics
	}

	metrics.kind = tableKind
	metrics.rows = int64(len(sheet.Table.Rows))
	for _, row := range sheet.Table.Rows {
		metrics.cols = max(metrics.cols, int64(len(row)))
		rowPresent := false
		for _, cell := range row {
			if cell == "" {
				metrics.emptyCells++
				continue
			}
			rowPresent = true
			metrics.presentCells++
			metrics.valueBytes += int64(len(cell))
		}
		if !rowPresent {
			metrics.emptyRows++
		}
	}
	metrics.missingCells = metrics.rows*metrics.cols - metrics.presentCells - metrics.emptyCells
	return metrics
}

// sheetMetrics stores accumulated measurements for one operation key.
type sheetMetrics struct {
	key      SheetMetricKey // key identifies the sheet parser operation.
	calls    int64          // calls counts parser invocations for the key.
	failures int64          // failures counts invocations that returned an error.
	cpuTime  time.Duration  // cpuTime is processor time sampled under the sheet's pprof label.
	wallTime time.Duration  // wallTime is elapsed parser time accumulated across calls.
	// input sums count and byte metrics across calls. cols and maxDepth retain
	// the largest observed value.
	input sheetInputMetrics
}

// sheetMetricsEntry guards the measurements stored for one operation key.
type sheetMetricsEntry struct {
	mu      sync.Mutex // mu protects metrics.
	metrics sheetMetrics
}

func (e *sheetMetricsEntry) snapshot() sheetMetrics {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.metrics
}

// SheetParserMetrics collects parser time and input metrics by sheet. Its zero
// value is ready for concurrent use.
type SheetParserMetrics struct {
	entries sync.Map // SheetMetricKey -> *sheetMetricsEntry
}

// Reset removes metrics collected by the previous generator run.
func (m *SheetParserMetrics) Reset() {
	m.entries.Clear()
}

// Measure runs parse with pprof labels and records its wall time, result, and
// input metrics. The CPU profile uses sheet_key to attribute processor time
// without pinning the goroutine to an OS thread.
func (m *SheetParserMetrics) Measure(ctx context.Context, generator string, key SheetMetricKey, sheet *book.Sheet, parse func(context.Context) error) (err error) {
	inputMetrics := measureSheetInput(sheet)
	var wall time.Duration
	labels := pprof.Labels(
		"generator", generator,
		"work", "sheet_parse",
		"book", key.Book,
		"sheet", key.Sheet,
		"detail", key.Detail,
		"sheet_key", key.String(),
	)
	pprof.Do(ctx, labels, func(ctx context.Context) {
		wallStart := time.Now()
		err = parse(ctx)
		wall = time.Since(wallStart)
	})
	m.record(key, inputMetrics, wall, err != nil)
	return err
}

func (m *SheetParserMetrics) record(key SheetMetricKey, input sheetInputMetrics, wall time.Duration, failed bool) {
	value, _ := m.entries.LoadOrStore(key, &sheetMetricsEntry{
		metrics: sheetMetrics{
			key:   key,
			input: sheetInputMetrics{kind: input.kind},
		},
	})
	entry := value.(*sheetMetricsEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	metrics := &entry.metrics
	metrics.calls++
	if failed {
		metrics.failures++
	}
	metrics.wallTime += wall
	metrics.input.rows += input.rows
	metrics.input.cols = max(metrics.input.cols, input.cols)
	metrics.input.presentCells += input.presentCells
	metrics.input.emptyCells += input.emptyCells
	metrics.input.missingCells += input.missingCells
	metrics.input.emptyRows += input.emptyRows
	metrics.input.valueBytes += input.valueBytes
	metrics.input.nodes += input.nodes
	metrics.input.scalarNodes += input.scalarNodes
	metrics.input.maxDepth = max(metrics.input.maxDepth, input.maxDepth)
}

func (m *SheetParserMetrics) collect() []sheetMetrics {
	var results []sheetMetrics
	m.entries.Range(func(_, value any) bool {
		results = append(results, value.(*sheetMetricsEntry).snapshot())
		return true
	})
	sort.Slice(results, func(i, j int) bool {
		if results[i].cpuTime != results[j].cpuTime {
			return results[i].cpuTime > results[j].cpuTime
		}
		if results[i].wallTime != results[j].wallTime {
			return results[i].wallTime > results[j].wallTime
		}
		return results[i].key.String() < results[j].key.String()
	})
	return results
}

// LoadCPUProfile attributes sampled processor time to sheets through the
// sheet_key pprof label. Short parser calls may have no samples and remain zero.
func (m *SheetParserMetrics) LoadCPUProfile(filename string) (err error) {
	if filename == "" {
		return nil
	}
	file, err := os.Open(filename)
	if err != nil {
		return xerrors.Wrapf(err, "open CPU profile %s", filename)
	}
	defer func() {
		err = errors.Join(err, xerrors.Wrapf(file.Close(), "close CPU profile %s", filename))
	}()

	parsed, err := profiledata.Parse(file)
	if err != nil {
		return xerrors.Wrapf(err, "parse CPU profile %s", filename)
	}
	cpuSampleIndex := -1
	for i, sampleType := range parsed.SampleType {
		if sampleType.Type == "cpu" && sampleType.Unit == "nanoseconds" {
			cpuSampleIndex = i
			break
		}
	}
	if cpuSampleIndex < 0 {
		return xerrors.Newf("CPU profile %s has no cpu/nanoseconds samples", filename)
	}

	cpuTimeBySheet := make(map[string]time.Duration)
	for _, sample := range parsed.Sample {
		if cpuSampleIndex >= len(sample.Value) {
			continue
		}
		cpuTime := time.Duration(sample.Value[cpuSampleIndex])
		for _, key := range sample.Label["sheet_key"] {
			cpuTimeBySheet[key] += cpuTime
		}
	}
	m.entries.Range(func(_, value any) bool {
		entry := value.(*sheetMetricsEntry)
		entry.mu.Lock()
		entry.metrics.cpuTime = cpuTimeBySheet[entry.metrics.key.String()]
		entry.mu.Unlock()
		return true
	})
	return nil
}

// Print reports each sheet's sampled CPU time, wall time, and input metrics.
func (m *SheetParserMetrics) Print() {
	results := m.collect()
	if len(results) == 0 {
		return
	}
	log.Infof("sheet parser metrics, sampled CPU time first (pprof label: sheet_key):")
	log.Info(formatSheetMetrics(results))
}

func formatSheetMetrics(results []sheetMetrics) string {
	var output strings.Builder
	writeTableSheetMetrics(&output, results)
	writeDocumentSheetMetrics(&output, results)
	return strings.TrimRight(output.String(), "\n")
}

func writeTableSheetMetrics(output *strings.Builder, results []sheetMetrics) {
	if !hasSheetKind(results, tableKind) {
		return
	}
	output.WriteString("table sheets:\n")
	w := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "RANK\tSHEET\tCPU TIME\tWALL TIME\tCALLS\tFAILURES\tROWS\tMAX COLS\tCELLS\tPRESENT\tABSENT\tEMPTY\tMISSING\tEMPTY ROWS\tVALUE BYTES")
	for i, result := range results {
		input := result.input
		if input.kind != tableKind {
			continue
		}
		absent := input.emptyCells + input.missingCells
		cells := input.presentCells + absent
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
			i+1, result.key, result.cpuTime, result.wallTime, result.calls, result.failures,
			input.rows, input.cols, cells, input.presentCells, absent,
			input.emptyCells, input.missingCells, input.emptyRows, input.valueBytes)
	}
	_ = w.Flush()
}

func writeDocumentSheetMetrics(output *strings.Builder, results []sheetMetrics) {
	if !hasSheetKind(results, documentKind) {
		return
	}
	if output.Len() > 0 {
		output.WriteByte('\n')
	}
	output.WriteString("document sheets:\n")
	w := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "RANK\tSHEET\tCPU TIME\tWALL TIME\tCALLS\tFAILURES\tNODES\tSCALAR NODES\tMAX DEPTH\tVALUE BYTES")
	for i, result := range results {
		input := result.input
		if input.kind != documentKind {
			continue
		}
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\n",
			i+1, result.key, result.cpuTime, result.wallTime, result.calls, result.failures,
			input.nodes, input.scalarNodes, input.maxDepth, input.valueBytes)
	}
	_ = w.Flush()
}

func hasSheetKind(results []sheetMetrics, kind sheetKind) bool {
	for _, result := range results {
		if result.input.kind == kind {
			return true
		}
	}
	return false
}
