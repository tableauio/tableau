package profile

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tableauio/tableau/internal/importer/book"
)

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

func TestSheetParserMetricsSortsByCPU(t *testing.T) {
	var metrics SheetParserMetrics
	shape := sheetShape{kind: "table", rows: 2, cols: 3, presentCells: 3, missingCells: 3}
	hot := SheetMetricKey{Book: "hot.xlsx", Sheet: "Items", Detail: "test.Items"}
	slowWall := SheetMetricKey{Book: "slow.xlsx", Sheet: "Items", Detail: "test.Items"}

	metrics.record(hot, shape, time.Second, 100*time.Millisecond, true, false)
	metrics.record(slowWall, shape, 3*time.Second, 10*time.Millisecond, true, false)
	metrics.record(hot, shape, 2*time.Second, 50*time.Millisecond, true, true)

	got := metrics.collect()
	if len(got) != 2 || got[0].key != hot || got[1].key != slowWall {
		t.Fatalf("CPU order = %+v, want hot then slowWall", got)
	}
	if got[0].calls != 2 || got[0].cpuCalls != 2 || got[0].failures != 1 ||
		got[0].cpuTime != 150*time.Millisecond || got[0].wallTime != 3*time.Second ||
		got[0].rows != 4 || got[0].cols != 3 || got[0].presentCells != 6 || got[0].missingCells != 6 {
		t.Errorf("aggregated metrics = %+v", got[0])
	}
}

func TestSheetParserMetricsFallsBackToWallTime(t *testing.T) {
	var metrics SheetParserMetrics
	shape := sheetShape{kind: "document", nodes: 1}
	fast := SheetMetricKey{Book: "fast.yaml", Sheet: "A", Detail: "test.A"}
	slow := SheetMetricKey{Book: "slow.yaml", Sheet: "B", Detail: "test.B"}
	metrics.record(fast, shape, time.Second, 0, false, false)
	metrics.record(slow, shape, 2*time.Second, 0, false, false)

	got := metrics.collect()
	if len(got) != 2 || got[0].key != slow || got[1].key != fast {
		t.Errorf("wall order = %+v, want slow then fast", got)
	}
}

func TestFormatSheetMetric(t *testing.T) {
	t.Run("table with CPU time", func(t *testing.T) {
		result := sheetMetricSnapshot{
			key:          SheetMetricKey{Book: "items.xlsx", Sheet: "Items", Detail: "test.Items"},
			kind:         "table",
			calls:        1,
			wallTime:     time.Second,
			cpuTime:      500 * time.Millisecond,
			cpuCalls:     1,
			rows:         2,
			cols:         3,
			presentCells: 4,
			emptyCells:   1,
			missingCells: 1,
			emptyRows:    1,
			valueBytes:   16,
		}
		got := formatSheetMetric(1, result)
		for _, want := range []string{
			"cpu=500ms wall=1s cpuCalls=1/1 failures=0",
			"rows=2 maxCols=3 cells=6 present=4 absent=2 (empty=1 missing=1)",
			"emptyRows=1 valueBytes=16",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("formatSheetMetric() = %q, want substring %q", got, want)
			}
		}
	})

	t.Run("document without CPU time", func(t *testing.T) {
		result := sheetMetricSnapshot{
			key:         SheetMetricKey{Book: "items.yaml", Sheet: "Items", Detail: "test.Items"},
			kind:        "document",
			calls:       1,
			failures:    1,
			wallTime:    time.Second,
			nodes:       4,
			scalarNodes: 2,
			maxDepth:    3,
			valueBytes:  16,
		}
		got := formatSheetMetric(1, result)
		for _, want := range []string{
			"cpu=n/a wall=1s cpuCalls=0/1 failures=1",
			"nodes=4 scalarNodes=2 maxDepth=3 valueBytes=16",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("formatSheetMetric() = %q, want substring %q", got, want)
			}
		}
	})
}

func TestSheetParserMetricsConcurrent(t *testing.T) {
	var metrics SheetParserMetrics
	key := SheetMetricKey{Book: "shared.xlsx", Sheet: "Items", Detail: "test.Items"}
	shape := sheetShape{kind: "table", rows: 1, cols: 1, presentCells: 1}
	const calls = 64

	var group sync.WaitGroup
	for range calls {
		group.Add(1)
		go func() {
			defer group.Done()
			metrics.record(key, shape, time.Millisecond, time.Millisecond, true, false)
		}()
	}
	group.Wait()

	got := metrics.collect()
	if len(got) != 1 || got[0].calls != calls || got[0].cpuCalls != calls ||
		got[0].wallTime != calls*time.Millisecond || got[0].cpuTime != calls*time.Millisecond ||
		got[0].presentCells != calls {
		t.Errorf("concurrent metrics = %+v", got)
	}
}

func TestSheetParserMetricsMeasureCPUTime(t *testing.T) {
	var metrics SheetParserMetrics
	sheet := book.NewTableSheet("Items", [][]string{{"ID"}})
	key := SheetMetricKey{Book: "items.xlsx", Sheet: "Items", Detail: "test.Items"}
	if err := metrics.Measure(context.Background(), "test", key, sheet, func(context.Context) error {
		deadline := time.Now().Add(80 * time.Millisecond)
		for time.Now().Before(deadline) {
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got := metrics.collect()
	if len(got) != 1 {
		t.Fatalf("metrics count = %d, want 1", len(got))
	}
	if got[0].cpuCalls == 0 {
		if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
			t.Fatal("thread CPU time unavailable on a supported platform")
		}
		if runtime.GOOS == "windows" {
			return
		}
		t.Skip("thread CPU time unavailable")
	}
	if got[0].wallTime <= 0 || got[0].cpuTime < 0 {
		t.Errorf("wall=%s, cpu=%s; wall should be positive and CPU nonnegative", got[0].wallTime, got[0].cpuTime)
	}
}

func TestSheetParserMetricsMeasurePreservesContext(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "value")
	var metrics SheetParserMetrics
	sheet := book.NewTableSheet("Items", [][]string{{"ID"}})
	key := SheetMetricKey{Book: "items.xlsx", Sheet: "Items", Detail: "test.Items"}

	if err := metrics.Measure(ctx, "test", key, sheet, func(ctx context.Context) error {
		if got := ctx.Value(contextKey{}); got != "value" {
			t.Fatalf("context value = %v, want value", got)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
