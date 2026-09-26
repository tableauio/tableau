package confgen

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/options"
)

func TestProfilingDisabled(t *testing.T) {
	gen := &Generator{OutputDir: t.TempDir()}

	stop, err := gen.startProfiling()
	if err != nil {
		t.Fatal(err)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(gen.OutputDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("disabled profiling wrote %d files", len(entries))
	}
}

func TestProfilingUsesRuntimeOption(t *testing.T) {
	outputDir := t.TempDir()
	opts := options.NewDefault()
	opts.Profiling = true
	opts.Conf.Output.Subdir = "profiles"
	gen := NewGeneratorWithOptions("", "", outputDir, opts)

	stop, err := gen.startProfiling()
	if err != nil {
		t.Fatal(err)
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"confgen-cpu.pprof", "confgen-mem.pprof"} {
		info, err := os.Stat(filepath.Join(outputDir, "profiles", name))
		if err != nil {
			t.Errorf("stat profile %s: %v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("profile %s is empty", name)
		}
	}
}

func TestMeasureSheet(t *testing.T) {
	t.Run("table with empty and missing cells", func(t *testing.T) {
		sheet := book.NewTableSheet("Items", [][]string{
			{"ID", "Name", ""},
			{"1", "", "Sword"},
			{"2"},
			{},
		})
		got := measureSheet(sheet)
		want := sheetShape{
			kind:         "table",
			rows:         4,
			cols:         3,
			presentCells: 5,
			emptyCells:   2,
			missingCells: 5,
			emptyRows:    1,
			valueBytes:   13,
		}
		if got != want {
			t.Errorf("measureSheet() = %+v, want %+v", got, want)
		}
	})

	t.Run("document nodes", func(t *testing.T) {
		sheet := book.NewDocumentSheet("Items", &book.Node{
			Kind: book.DocumentNode,
			Children: []*book.Node{{
				Kind: book.MapNode,
				Children: []*book.Node{
					{Kind: book.ScalarNode, Value: "Sword"},
					{Kind: book.ScalarNode},
				},
			}},
		})
		got := measureSheet(sheet)
		want := sheetShape{
			kind:        "document",
			nodes:       4,
			scalarNodes: 2,
			maxDepth:    3,
			valueBytes:  5,
		}
		if got != want {
			t.Errorf("measureSheet() = %+v, want %+v", got, want)
		}
	})
}

func TestCollectSheetMetricsSortsByCPU(t *testing.T) {
	gen := &Generator{}
	shape := sheetShape{kind: "table", rows: 2, cols: 3, presentCells: 3, missingCells: 3}
	hot := sheetMetricKey{book: "hot.xlsx", sheet: "Items", message: "test.Items"}
	slowWall := sheetMetricKey{book: "slow.xlsx", sheet: "Items", message: "test.Items"}

	recordSheetMetrics(&gen.SheetParserMetrics, hot, shape, time.Second, 100*time.Millisecond, true, false)
	recordSheetMetrics(&gen.SheetParserMetrics, slowWall, shape, 3*time.Second, 10*time.Millisecond, true, false)
	recordSheetMetrics(&gen.SheetParserMetrics, hot, shape, 2*time.Second, 50*time.Millisecond, true, true)

	got := collectSheetMetrics(gen)
	if len(got) != 2 || got[0].key != hot || got[1].key != slowWall {
		t.Fatalf("CPU order = %+v, want hot then slowWall", got)
	}
	if got[0].calls != 2 || got[0].cpuCalls != 2 || got[0].failures != 1 ||
		got[0].cpuTime != 150*time.Millisecond || got[0].wallTime != 3*time.Second ||
		got[0].rows != 4 || got[0].cols != 3 || got[0].presentCells != 6 || got[0].missingCells != 6 {
		t.Errorf("aggregated metrics = %+v", got[0])
	}
}

func TestCollectSheetMetricsFallsBackToWallTime(t *testing.T) {
	gen := &Generator{}
	shape := sheetShape{kind: "document", nodes: 1}
	fast := sheetMetricKey{book: "fast.yaml", sheet: "A", message: "test.A"}
	slow := sheetMetricKey{book: "slow.yaml", sheet: "B", message: "test.B"}
	recordSheetMetrics(&gen.SheetParserMetrics, fast, shape, time.Second, 0, false, false)
	recordSheetMetrics(&gen.SheetParserMetrics, slow, shape, 2*time.Second, 0, false, false)

	got := collectSheetMetrics(gen)
	if len(got) != 2 || got[0].key != slow || got[1].key != fast {
		t.Errorf("wall order = %+v, want slow then fast", got)
	}
}

func TestPrintSheetMetrics(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		PrintSheetMetrics(&Generator{})
	})

	t.Run("table with CPU time", func(t *testing.T) {
		gen := &Generator{}
		recordSheetMetrics(
			&gen.SheetParserMetrics,
			sheetMetricKey{book: "items.xlsx", sheet: "Items", message: "test.Items"},
			sheetShape{
				kind:         "table",
				rows:         2,
				cols:         3,
				presentCells: 4,
				emptyCells:   1,
				missingCells: 1,
				emptyRows:    1,
				valueBytes:   16,
			},
			time.Second,
			500*time.Millisecond,
			true,
			false,
		)
		results := collectSheetMetrics(gen)
		got := formatSheetMetrics(1, results[0])
		for _, want := range []string{
			"cpu=500ms wall=1s cpuCalls=1/1 failures=0",
			"rows=2 maxCols=3 cells=6 present=4 absent=2 (empty=1 missing=1)",
			"emptyRows=1 valueBytes=16",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("formatSheetMetrics() = %q, want substring %q", got, want)
			}
		}
		PrintSheetMetrics(gen)
	})

	t.Run("document without CPU time", func(t *testing.T) {
		gen := &Generator{}
		recordSheetMetrics(
			&gen.SheetParserMetrics,
			sheetMetricKey{book: "items.yaml", sheet: "Items", message: "test.Items"},
			sheetShape{
				kind:        "document",
				nodes:       4,
				scalarNodes: 2,
				maxDepth:    3,
				valueBytes:  16,
			},
			time.Second,
			0,
			false,
			true,
		)
		results := collectSheetMetrics(gen)
		got := formatSheetMetrics(1, results[0])
		for _, want := range []string{
			"cpu=n/a wall=1s cpuCalls=0/1 failures=1",
			"nodes=4 scalarNodes=2 maxDepth=3 valueBytes=16",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("formatSheetMetrics() = %q, want substring %q", got, want)
			}
		}
		PrintSheetMetrics(gen)
	})
}

func TestRecordSheetMetricsConcurrent(t *testing.T) {
	gen := &Generator{}
	key := sheetMetricKey{book: "shared.xlsx", sheet: "Items", message: "test.Items"}
	shape := sheetShape{kind: "table", rows: 1, cols: 1, presentCells: 1}
	const calls = 64

	var group sync.WaitGroup
	for range calls {
		group.Add(1)
		go func() {
			defer group.Done()
			recordSheetMetrics(&gen.SheetParserMetrics, key, shape, time.Millisecond, time.Millisecond, true, false)
		}()
	}
	group.Wait()

	got := collectSheetMetrics(gen)
	if len(got) != 1 || got[0].calls != calls || got[0].cpuCalls != calls ||
		got[0].wallTime != calls*time.Millisecond || got[0].cpuTime != calls*time.Millisecond ||
		got[0].presentCells != calls {
		t.Errorf("concurrent metrics = %+v", got)
	}
}

func TestMeasureSheetParseCPUTime(t *testing.T) {
	key := sheetMetricKey{book: "items.xlsx", sheet: "Items", message: "test.Items"}
	wall, cpu, available, err := measureSheetParse(key, func() error {
		deadline := time.Now().Add(80 * time.Millisecond)
		for time.Now().Before(deadline) {
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !available {
		if runtime.GOOS == "windows" || runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
			t.Fatal("thread CPU time unavailable on a supported platform")
		}
		t.Skip("thread CPU time unavailable")
	}
	if wall <= 0 || cpu < 0 {
		t.Errorf("wall=%s, cpu=%s; wall should be positive and CPU nonnegative", wall, cpu)
	}
}
