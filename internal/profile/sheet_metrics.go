package profile

import (
	"context"
	"fmt"
	"runtime"
	"runtime/pprof"
	"sort"
	"sync"
	"time"

	"github.com/tableauio/tableau/internal/importer/book"
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
	wallTime     time.Duration
	cpuTime      time.Duration
	cpuCalls     int64
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

// Measure runs parse with pprof labels and records its wall time, processor
// time, result, and input shape. Platforms with a precise thread CPU clock pin
// the parse to its current thread while measuring processor time.
func (m *SheetParserMetrics) Measure(ctx context.Context, generator string, key SheetMetricKey, sheet *book.Sheet, parse func(context.Context) error) (err error) {
	shape := measureSheet(sheet)
	var wall, cpu time.Duration
	var cpuMeasured bool
	labels := pprof.Labels(
		"generator", generator,
		"work", "sheet_parse",
		"book", key.Book,
		"sheet", key.Sheet,
		"detail", key.Detail,
	)
	pprof.Do(ctx, labels, func(ctx context.Context) {
		wallStart := time.Now()
		if supportsThreadCPUTime {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()

			cpuStart, startOK := MeasureThreadCPUTime()
			err = parse(ctx)
			cpuEnd, endOK := MeasureThreadCPUTime()
			if startOK && endOK && cpuEnd >= cpuStart {
				cpu = cpuEnd - cpuStart
				cpuMeasured = true
			}
		} else {
			err = parse(ctx)
		}
		wall = time.Since(wallStart)
	})
	m.record(key, shape, wall, cpu, cpuMeasured, err != nil)
	return err
}

func (m *SheetParserMetrics) record(key SheetMetricKey, shape sheetShape, wall, cpu time.Duration, cpuMeasured, failed bool) {
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
	if cpuMeasured {
		entry.cpuTime += cpu
		entry.cpuCalls++
	}
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
	cpuAvailable := false
	for _, result := range results {
		if result.cpuCalls > 0 {
			cpuAvailable = true
			break
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if cpuAvailable {
			if (results[i].cpuCalls > 0) != (results[j].cpuCalls > 0) {
				return results[i].cpuCalls > 0
			}
			if results[i].cpuTime != results[j].cpuTime {
				return results[i].cpuTime > results[j].cpuTime
			}
		}
		if results[i].wallTime != results[j].wallTime {
			return results[i].wallTime > results[j].wallTime
		}
		return results[i].key.String() < results[j].key.String()
	})
	return results
}

// Print reports each sheet's parser CPU and wall time, sorted by CPU time when
// available and then by wall time.
func (m *SheetParserMetrics) Print() {
	results := m.collect()
	if len(results) == 0 {
		return
	}
	if results[0].cpuCalls > 0 {
		log.Infof("sheet parser CPU time, slowest first (wall time may overlap):")
	} else {
		log.Infof("sheet parser wall time, slowest first (thread CPU time unavailable):")
	}
	for i, result := range results {
		log.Info(formatSheetMetric(i+1, result))
	}
}

func formatSheetMetric(rank int, result sheetMetricSnapshot) string {
	cpuText := "n/a"
	if result.cpuCalls > 0 {
		cpuText = result.cpuTime.String()
	}
	if result.kind == "table" {
		absent := result.emptyCells + result.missingCells
		cells := result.presentCells + absent
		return fmt.Sprintf("%3d. %s: cpu=%s wall=%s cpuCalls=%d/%d failures=%d rows=%d maxCols=%d cells=%d present=%d absent=%d (empty=%d missing=%d) emptyRows=%d valueBytes=%d",
			rank, result.key, cpuText, result.wallTime, result.cpuCalls, result.calls, result.failures,
			result.rows, result.cols, cells, result.presentCells, absent,
			result.emptyCells, result.missingCells, result.emptyRows, result.valueBytes)
	}
	return fmt.Sprintf("%3d. %s: cpu=%s wall=%s cpuCalls=%d/%d failures=%d nodes=%d scalarNodes=%d maxDepth=%d valueBytes=%d",
		rank, result.key, cpuText, result.wallTime, result.cpuCalls, result.calls, result.failures,
		result.nodes, result.scalarNodes, result.maxDepth, result.valueBytes)
}
