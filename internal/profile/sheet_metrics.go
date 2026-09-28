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
	Book   string
	Sheet  string
	Detail string
}

func (k SheetMetricKey) String() string {
	name := k.Book + "#" + k.Sheet
	if k.Detail == "" {
		return name
	}
	return name + " (" + k.Detail + ")"
}

type sheetShape struct {
	kind         string
	rows         int64
	cols         int64
	presentCells int64
	emptyCells   int64
	missingCells int64
	emptyRows    int64
	valueBytes   int64
	nodes        int64
	scalarNodes  int64
	maxDepth     int64
}

// measureSheet describes the imported sheet before parsing can add virtual
// nodes. Missing table cells are the implicit cells after short rows up to the
// widest row. Empty cells are explicitly stored empty strings.
func measureSheet(sheet *book.Sheet) sheetShape {
	var shape sheetShape
	if sheet.Document != nil {
		shape.kind = "document"
		var visit func(*book.Node, int64)
		visit = func(node *book.Node, depth int64) {
			if node == nil {
				return
			}
			shape.nodes++
			shape.maxDepth = max(shape.maxDepth, depth)
			if node.Kind == book.ScalarNode {
				shape.scalarNodes++
				shape.valueBytes += int64(len(node.Value))
			}
			for _, child := range node.Children {
				visit(child, depth+1)
			}
		}
		visit(sheet.Document, 1)
		return shape
	}
	if sheet.Table == nil {
		return shape
	}

	shape.kind = "table"
	shape.rows = int64(len(sheet.Table.Rows))
	for _, row := range sheet.Table.Rows {
		shape.cols = max(shape.cols, int64(len(row)))
		rowPresent := false
		for _, cell := range row {
			if cell == "" {
				shape.emptyCells++
				continue
			}
			rowPresent = true
			shape.presentCells++
			shape.valueBytes += int64(len(cell))
		}
		if !rowPresent {
			shape.emptyRows++
		}
	}
	shape.missingCells = shape.rows*shape.cols - shape.presentCells - shape.emptyCells
	return shape
}

type sheetMetricSnapshot struct {
	key          SheetMetricKey
	kind         string
	calls        int64
	failures     int64
	cpuTime      time.Duration
	wallTime     time.Duration
	rows         int64
	cols         int64
	presentCells int64
	emptyCells   int64
	missingCells int64
	emptyRows    int64
	valueBytes   int64
	nodes        int64
	scalarNodes  int64
	maxDepth     int64
}

type sheetMetric struct {
	mu sync.Mutex
	sheetMetricSnapshot
}

func (m *sheetMetric) snapshot() sheetMetricSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sheetMetricSnapshot
}

// SheetParserMetrics collects parser time and input shape by sheet. Its zero
// value is ready for concurrent use.
type SheetParserMetrics struct {
	entries sync.Map
}

// Reset removes metrics collected by the previous generator run.
func (m *SheetParserMetrics) Reset() {
	m.entries.Clear()
}

// Measure runs parse with pprof labels and records its wall time, result, and
// input shape. The CPU profile uses sheet_key to attribute processor time
// without pinning the goroutine to an OS thread.
func (m *SheetParserMetrics) Measure(ctx context.Context, generator string, key SheetMetricKey, sheet *book.Sheet, parse func(context.Context) error) (err error) {
	shape := measureSheet(sheet)
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
	m.record(key, shape, wall, err != nil)
	return err
}

func (m *SheetParserMetrics) record(key SheetMetricKey, shape sheetShape, wall time.Duration, failed bool) {
	value, _ := m.entries.LoadOrStore(key, &sheetMetric{
		sheetMetricSnapshot: sheetMetricSnapshot{key: key, kind: shape.kind},
	})
	entry := value.(*sheetMetric)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	entry.calls++
	if failed {
		entry.failures++
	}
	entry.wallTime += wall
	entry.rows += shape.rows
	entry.cols = max(entry.cols, shape.cols)
	entry.presentCells += shape.presentCells
	entry.emptyCells += shape.emptyCells
	entry.missingCells += shape.missingCells
	entry.emptyRows += shape.emptyRows
	entry.valueBytes += shape.valueBytes
	entry.nodes += shape.nodes
	entry.scalarNodes += shape.scalarNodes
	entry.maxDepth = max(entry.maxDepth, shape.maxDepth)
}

func (m *SheetParserMetrics) collect() []sheetMetricSnapshot {
	var results []sheetMetricSnapshot
	m.entries.Range(func(_, value any) bool {
		results = append(results, value.(*sheetMetric).snapshot())
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
	cpuIndex := -1
	for i, sampleType := range parsed.SampleType {
		if sampleType.Type == "cpu" && sampleType.Unit == "nanoseconds" {
			cpuIndex = i
			break
		}
	}
	if cpuIndex < 0 {
		return xerrors.Newf("CPU profile %s has no cpu/nanoseconds samples", filename)
	}

	cpuBySheet := make(map[string]time.Duration)
	for _, sample := range parsed.Sample {
		if cpuIndex >= len(sample.Value) {
			continue
		}
		cpuTime := time.Duration(sample.Value[cpuIndex])
		for _, key := range sample.Label["sheet_key"] {
			cpuBySheet[key] += cpuTime
		}
	}
	m.entries.Range(func(_, value any) bool {
		entry := value.(*sheetMetric)
		entry.mu.Lock()
		entry.cpuTime = cpuBySheet[entry.key.String()]
		entry.mu.Unlock()
		return true
	})
	return nil
}

// Print reports each sheet's sampled CPU time, wall time, and input shape.
func (m *SheetParserMetrics) Print() {
	results := m.collect()
	if len(results) == 0 {
		return
	}
	log.Infof("sheet parser metrics, sampled CPU time first (pprof label: sheet_key):")
	log.Info(formatSheetMetrics(results))
}

func formatSheetMetrics(results []sheetMetricSnapshot) string {
	var output strings.Builder
	writeTableSheetMetrics(&output, results)
	writeDocumentSheetMetrics(&output, results)
	return strings.TrimRight(output.String(), "\n")
}

func writeTableSheetMetrics(output *strings.Builder, results []sheetMetricSnapshot) {
	if !hasSheetKind(results, "table") {
		return
	}
	output.WriteString("table sheets:\n")
	w := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "RANK\tSHEET\tCPU TIME\tWALL TIME\tCALLS\tFAILURES\tROWS\tMAX COLS\tCELLS\tPRESENT\tABSENT\tEMPTY\tMISSING\tEMPTY ROWS\tVALUE BYTES")
	for i, result := range results {
		if result.kind != "table" {
			continue
		}
		absent := result.emptyCells + result.missingCells
		cells := result.presentCells + absent
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
			i+1, result.key, result.cpuTime, result.wallTime, result.calls, result.failures,
			result.rows, result.cols, cells, result.presentCells, absent,
			result.emptyCells, result.missingCells, result.emptyRows, result.valueBytes)
	}
	_ = w.Flush()
}

func writeDocumentSheetMetrics(output *strings.Builder, results []sheetMetricSnapshot) {
	if !hasSheetKind(results, "document") {
		return
	}
	if output.Len() > 0 {
		output.WriteByte('\n')
	}
	output.WriteString("document sheets:\n")
	w := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "RANK\tSHEET\tCPU TIME\tWALL TIME\tCALLS\tFAILURES\tNODES\tSCALAR NODES\tMAX DEPTH\tVALUE BYTES")
	for i, result := range results {
		if result.kind != "document" {
			continue
		}
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\n",
			i+1, result.key, result.cpuTime, result.wallTime, result.calls, result.failures,
			result.nodes, result.scalarNodes, result.maxDepth, result.valueBytes)
	}
	_ = w.Flush()
}

func hasSheetKind(results []sheetMetricSnapshot, kind string) bool {
	for _, result := range results {
		if result.kind == kind {
			return true
		}
	}
	return false
}
