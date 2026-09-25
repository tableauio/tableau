package confgen

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sort"
	"sync"
	"time"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/profile"
	"github.com/tableauio/tableau/log"
)

func (gen *Generator) startProfiling() (func() error, error) {
	if !gen.enableProfiling {
		return func() error { return nil }, nil
	}

	profileDir := gen.OutputDir
	if gen.OutputOpt != nil {
		profileDir = filepath.Join(profileDir, gen.OutputOpt.Subdir)
	}
	return profile.Start("confgen", profileDir)
}

type sheetPerfKey struct {
	book    string
	sheet   string
	message string
}

func (k sheetPerfKey) String() string {
	return k.book + "#" + k.sheet + " (" + k.message + ")"
}

// measureSheetParse measures elapsed and processor time on the parser's OS thread.
// A CPU profile also attributes samples to this sheet through confgen_sheet.
func measureSheetParse(key sheetPerfKey, parse func() error) (wall, cpu time.Duration, cpuMeasured bool, err error) {
	pprof.Do(context.Background(), pprof.Labels("confgen_sheet", key.String()), func(context.Context) {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		cpuStart, startOK := profile.MeasureThreadCPUTime()
		wallStart := time.Now()
		err = parse()
		wall = time.Since(wallStart)
		cpuEnd, endOK := profile.MeasureThreadCPUTime()
		if startOK && endOK && cpuEnd >= cpuStart {
			cpu = cpuEnd - cpuStart
			cpuMeasured = true
		}
	})
	return
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

// measureSheet describes the imported sheet before parsing can add virtual nodes.
// For tables, missing cells are the implicit cells after short rows up to the
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

type sheetPerfSnapshot struct {
	key          sheetPerfKey
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

type sheetPerfStats struct {
	mu sync.Mutex
	sheetPerfSnapshot
}

func (s *sheetPerfStats) snapshot() sheetPerfSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sheetPerfSnapshot
}

func recordSheetPerf(stats *sync.Map, key sheetPerfKey, shape sheetShape, elapsed, cpu time.Duration, cpuMeasured, failed bool) {
	if stats == nil {
		return
	}
	value, _ := stats.LoadOrStore(key, &sheetPerfStats{
		sheetPerfSnapshot: sheetPerfSnapshot{key: key, kind: shape.kind},
	})
	entry := value.(*sheetPerfStats)
	entry.mu.Lock()
	defer entry.mu.Unlock()

	entry.calls++
	if failed {
		entry.failures++
	}
	entry.wallTime += elapsed
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

func collectSheetPerf(gen *Generator) []sheetPerfSnapshot {
	var results []sheetPerfSnapshot
	gen.SheetParserStats.Range(func(_, value any) bool {
		results = append(results, value.(*sheetPerfStats).snapshot())
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

// PrintPerfStats reports each sheet's parser CPU and wall time. Results are
// sorted by CPU time where available, then by wall time.
func PrintPerfStats(gen *Generator) {
	results := collectSheetPerf(gen)
	if len(results) == 0 {
		return
	}
	if results[0].cpuCalls > 0 {
		log.Infof("sheet parser CPU time, slowest first (wall time may overlap):")
	} else {
		log.Infof("sheet parser wall time, slowest first (thread CPU time unavailable):")
	}
	for i, result := range results {
		log.Info(formatSheetPerf(i+1, result))
	}
}

func formatSheetPerf(rank int, result sheetPerfSnapshot) string {
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
