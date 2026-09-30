package profile

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/tableauio/tableau/log"
)

type bookMetric struct {
	name          string
	calls         int64
	failures      int64
	cpuTime       time.Duration
	totalWallTime time.Duration
	maxWallTime   time.Duration
}

// BookMetrics collects cumulative and maximum wall time for named generator
// operations. Its zero value is ready for concurrent use.
type BookMetrics struct {
	store metricStore[string, bookMetric]
}

// Reset removes metrics from a previous run.
func (m *BookMetrics) Reset() {
	m.store.clear()
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
	m.store.update(name, bookMetric{name: name}, func(metric *bookMetric) {
		metric.calls++
		if failed {
			metric.failures++
		}
		metric.totalWallTime += elapsed
		metric.maxWallTime = max(metric.maxWallTime, elapsed)
	})
}

func (m *BookMetrics) snapshot() []bookMetric {
	metrics := m.store.snapshot()
	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].totalWallTime != metrics[j].totalWallTime {
			return metrics[i].totalWallTime > metrics[j].totalWallTime
		}
		return metrics[i].name < metrics[j].name
	})
	return metrics
}

// LoadCPUProfile attributes sampled CPU time to operation names.
func (m *BookMetrics) LoadCPUProfile(filename string) error {
	if filename == "" {
		return nil
	}
	samples, err := cpuTimeByLabel(filename, "name")
	if err != nil {
		return err
	}
	m.store.each(func(metric *bookMetric) {
		metric.cpuTime = samples[metric.name]
	})
	return nil
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
	_, _ = fmt.Fprintln(w, "RANK	NAME	CPU TIME	TOTAL WALL TIME	MAX WALL TIME	CALLS	FAILURES")
	for i, metric := range metrics {
		_, _ = fmt.Fprintf(w, "%d	%s	%s	%s	%s	%d	%d\n",
			i+1, metric.name, metric.cpuTime, metric.totalWallTime, metric.maxWallTime, metric.calls, metric.failures)
	}
	_ = w.Flush()
	log.Infof("generator book metrics, cumulative wall time first (concurrent calls overlap):")
	log.Info(strings.TrimRight(output.String(), "\n"))
}
