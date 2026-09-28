package profile

import (
	"context"
	"errors"
	"fmt"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/log/core"
)

type metricLogDriver struct {
	messages []string
}

func (*metricLogDriver) Name() string { return "metrics-test" }

func (*metricLogDriver) GetLevel(string) core.Level { return core.DebugLevel }

func (d *metricLogDriver) Print(record *core.Record) {
	message := *record.Format
	if message == "" {
		message = fmt.Sprint(record.Args...)
	} else if len(record.Args) > 0 {
		message = fmt.Sprintf(message, record.Args...)
	}
	d.messages = append(d.messages, message)
}

func TestMeasureSheetInput(t *testing.T) {
	t.Run("table with empty and missing cells", func(t *testing.T) {
		sheet := book.NewTableSheet("Items", [][]string{
			{"ID", "Name", ""},
			{"1", "", "Sword"},
			{"2"},
			{},
		})
		got := measureSheetInput(sheet)
		want := sheetInputMetrics{
			kind:         tableKind,
			rows:         4,
			cols:         3,
			presentCells: 5,
			emptyCells:   2,
			missingCells: 5,
			emptyRows:    1,
			valueBytes:   13,
		}
		if got != want {
			t.Errorf("measureSheetInput() = %+v, want %+v", got, want)
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
		got := measureSheetInput(sheet)
		want := sheetInputMetrics{
			kind:        documentKind,
			nodes:       4,
			scalarNodes: 2,
			maxDepth:    3,
			valueBytes:  5,
		}
		if got != want {
			t.Errorf("measureSheetInput() = %+v, want %+v", got, want)
		}
	})
}

func TestSheetParserMetricsSortsByWallTime(t *testing.T) {
	var metrics SheetParserMetrics
	input := sheetInputMetrics{kind: tableKind, rows: 2, cols: 3, presentCells: 3, missingCells: 3}
	fast := SheetMetricKey{Book: "fast.xlsx", Sheet: "Items", Detail: "test.Items"}
	slow := SheetMetricKey{Book: "slow.xlsx", Sheet: "Items", Detail: "test.Items"}

	metrics.record(fast, input, time.Second, false)
	metrics.record(slow, input, 3*time.Second, false)
	metrics.record(fast, input, time.Second, true)

	got := metrics.collect()
	if len(got) != 2 || got[0].key != slow || got[1].key != fast {
		t.Fatalf("wall order = %+v, want slow then fast", got)
	}
	if got[1].calls != 2 || got[1].failures != 1 || got[1].wallTime != 2*time.Second ||
		got[1].input.rows != 4 || got[1].input.cols != 3 || got[1].input.presentCells != 6 || got[1].input.missingCells != 6 {
		t.Errorf("aggregated metrics = %+v", got[1])
	}
}

func TestSheetParserMetricsBreaksWallTimeTiesByKey(t *testing.T) {
	var metrics SheetParserMetrics
	input := sheetInputMetrics{kind: documentKind, nodes: 1}
	a := SheetMetricKey{Book: "a.yaml", Sheet: "A", Detail: "test.A"}
	b := SheetMetricKey{Book: "b.yaml", Sheet: "B", Detail: "test.B"}
	metrics.record(b, input, time.Second, false)
	metrics.record(a, input, time.Second, false)

	got := metrics.collect()
	if len(got) != 2 || got[0].key != a || got[1].key != b {
		t.Errorf("key order = %+v, want a then b", got)
	}
}

func TestSheetParserMetricsPrintAndReset(t *testing.T) {
	driver := &metricLogDriver{}
	log.SetDriver(driver)
	t.Cleanup(func() { log.SetDriver(nil) })

	var metrics SheetParserMetrics
	metrics.Print()
	if len(driver.messages) != 0 {
		t.Fatalf("empty metrics logged %d messages, want 0", len(driver.messages))
	}

	key := SheetMetricKey{Book: "items.xlsx", Sheet: "Items", Detail: "test.Items"}
	metrics.record(key, sheetInputMetrics{kind: tableKind, rows: 1, cols: 1}, time.Second, false)
	metrics.Print()
	if len(driver.messages) != 2 || !strings.Contains(driver.messages[0], "sheet_key") || !strings.Contains(driver.messages[1], key.String()) {
		t.Fatalf("metrics log = %q", driver.messages)
	}

	metrics.Reset()
	if got := metrics.collect(); len(got) != 0 {
		t.Fatalf("metrics after Reset() = %v, want empty", got)
	}
	driver.messages = nil
	metrics.record(key, sheetInputMetrics{kind: tableKind}, time.Second, false)
	metrics.Print()
	if len(driver.messages) != 2 || !strings.Contains(driver.messages[0], "CPU time") {
		t.Fatalf("CPU metrics log = %q", driver.messages)
	}
}

func TestFormatSheetMetrics(t *testing.T) {
	t.Run("table", func(t *testing.T) {
		result := sheetMetrics{
			key:      SheetMetricKey{Book: "items.xlsx", Sheet: "Items", Detail: "test.Items"},
			calls:    1,
			cpuTime:  250 * time.Millisecond,
			wallTime: time.Second,
			input: sheetInputMetrics{
				kind:         tableKind,
				rows:         2,
				cols:         3,
				presentCells: 4,
				emptyCells:   1,
				missingCells: 1,
				emptyRows:    1,
				valueBytes:   16,
			},
		}
		got := formatSheetMetrics([]sheetMetrics{result})
		for _, want := range []string{
			"RANK",
			"SHEET",
			"items.xlsx#Items (test.Items)",
			"250ms",
			"1s",
			"2",
			"3",
			"6",
			"16",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("formatSheetMetrics() = %q, want substring %q", got, want)
			}
		}
	})

	t.Run("document", func(t *testing.T) {
		result := sheetMetrics{
			key:      SheetMetricKey{Book: "items.yaml", Sheet: "Items", Detail: "test.Items"},
			calls:    1,
			failures: 1,
			cpuTime:  500 * time.Millisecond,
			wallTime: time.Second,
			input: sheetInputMetrics{
				kind:        documentKind,
				nodes:       4,
				scalarNodes: 2,
				maxDepth:    3,
				valueBytes:  16,
			},
		}
		got := formatSheetMetrics([]sheetMetrics{result})
		for _, want := range []string{
			"RANK",
			"SHEET",
			"items.yaml#Items (test.Items)",
			"500ms",
			"1s",
			"4",
			"2",
			"3",
			"16",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("formatSheetMetrics() = %q, want substring %q", got, want)
			}
		}
	})
}

func TestSheetParserMetricsConcurrent(t *testing.T) {
	var metrics SheetParserMetrics
	key := SheetMetricKey{Book: "shared.xlsx", Sheet: "Items", Detail: "test.Items"}
	input := sheetInputMetrics{kind: tableKind, rows: 1, cols: 1, presentCells: 1}
	const calls = 64

	var group sync.WaitGroup
	for range calls {
		group.Add(1)
		go func() {
			defer group.Done()
			metrics.record(key, input, time.Millisecond, false)
		}()
	}
	group.Wait()

	got := metrics.collect()
	if len(got) != 1 || got[0].calls != calls || got[0].wallTime != calls*time.Millisecond ||
		got[0].input.presentCells != calls {
		t.Errorf("concurrent metrics = %+v", got)
	}
}

func TestSheetParserMetricsMeasureLabelsCPUWork(t *testing.T) {
	var metrics SheetParserMetrics
	sheet := book.NewTableSheet("Items", [][]string{{"ID"}})
	key := SheetMetricKey{Book: "items.xlsx", Sheet: "Items", Detail: "test.Items"}
	if err := metrics.Measure(context.Background(), "test", key, sheet, func(ctx context.Context) error {
		wantLabels := map[string]string{
			"generator": "test",
			"work":      "sheet_parse",
			"book":      key.Book,
			"sheet":     key.Sheet,
			"detail":    key.Detail,
			"sheet_key": key.String(),
		}
		for name, want := range wantLabels {
			if got, ok := pprof.Label(ctx, name); !ok || got != want {
				t.Errorf("label %q = %q, %t; want %q, true", name, got, ok, want)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got := metrics.collect()
	if len(got) != 1 {
		t.Fatalf("metrics count = %d, want 1", len(got))
	}
}

func TestSheetParserMetricsMeasureRecordsFailure(t *testing.T) {
	var metrics SheetParserMetrics
	sheet := book.NewTableSheet("Items", [][]string{{"ID"}})
	key := SheetMetricKey{Book: "items.xlsx", Sheet: "Items", Detail: "test.Items"}
	wantErr := errors.New("parse failed")
	err := metrics.Measure(context.Background(), "test", key, sheet, func(context.Context) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Measure() error = %v, want %v", err, wantErr)
	}
	got := metrics.collect()
	if len(got) != 1 || got[0].failures != 1 {
		t.Fatalf("failed metrics = %+v", got)
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
