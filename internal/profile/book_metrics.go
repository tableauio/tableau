package profile

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/tableauio/tableau/log"
)

type bookMetric struct {
	name          string
	calls         int64
	failures      int64
	totalWallTime time.Duration
	maxWallTime   time.Duration
}

type bookMetricEntry struct {
	mu     sync.Mutex
	metric bookMetric
}

// BookMetrics collects cumulative and maximum wall time for named generator
// operations. Its zero value is ready for concurrent use.
type BookMetrics struct {
	entries sync.Map // metric name -> *bookMetricEntry
}

// Measure runs an operation with pprof labels and records its elapsed time and result.
func (m *BookMetrics) Measure(ctx context.Context, generator, name string, operation func(context.Context) error, labels ...string) (err error) {
	start := time.Now()
	profileLabels := []string{"generator", generator, "name", name}
	profileLabels = append(profileLabels, labels...)
	err = Run(ctx, operation, profileLabels...)
	m.record(name, time.Since(start), err != nil)
	return err
}

func (m *BookMetrics) record(name string, elapsed time.Duration, failed bool) {
	stored, _ := m.entries.LoadOrStore(name, &bookMetricEntry{
		metric: bookMetric{name: name},
	})
	entry := stored.(*bookMetricEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.metric.calls++
	if failed {
		entry.metric.failures++
	}
	entry.metric.totalWallTime += elapsed
	entry.metric.maxWallTime = max(entry.metric.maxWallTime, elapsed)
}

func (m *BookMetrics) snapshot() []bookMetric {
	var metrics []bookMetric
	m.entries.Range(func(_, stored any) bool {
		entry := stored.(*bookMetricEntry)
		entry.mu.Lock()
		metrics = append(metrics, entry.metric)
		entry.mu.Unlock()
		return true
	})
	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].totalWallTime != metrics[j].totalWallTime {
			return metrics[i].totalWallTime > metrics[j].totalWallTime
		}
		return metrics[i].name < metrics[j].name
	})
	return metrics
}

// Print reports named book-pipeline operations ordered by cumulative wall time.
// Times from concurrent calls overlap and therefore do not sum to generator time.
func (m *BookMetrics) Print() {
	metrics := m.snapshot()
	if len(metrics) == 0 {
		return
	}
	var output strings.Builder
	w := tabwriter.NewWriter(&output, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "RANK	NAME	TOTAL WALL TIME	MAX WALL TIME	CALLS	FAILURES")
	for i, metric := range metrics {
		_, _ = fmt.Fprintf(w, "%d	%s	%s	%s	%d	%d\n",
			i+1, metric.name, metric.totalWallTime, metric.maxWallTime, metric.calls, metric.failures)
	}
	_ = w.Flush()
	log.Infof("generator book metrics, cumulative wall time first (concurrent calls overlap):")
	log.Info(strings.TrimRight(output.String(), "\n"))
}
