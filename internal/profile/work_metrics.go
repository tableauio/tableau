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

type workMetrics struct {
	work     string
	calls    int64
	failures int64
	wallTime time.Duration
	maxTime  time.Duration
}

type workMetricsEntry struct {
	mu      sync.Mutex
	metrics workMetrics
}

// WorkMetrics collects cumulative and maximum wall time for named generator
// operations. Its zero value is ready for concurrent use.
type WorkMetrics struct {
	entries sync.Map // work name -> *workMetricsEntry
}

// Measure runs work with pprof labels and records its elapsed time and result.
func (m *WorkMetrics) Measure(ctx context.Context, generator, work string, run func(context.Context) error, labels ...string) (err error) {
	start := time.Now()
	profileLabels := []string{"generator", generator, "work", work}
	profileLabels = append(profileLabels, labels...)
	err = Run(ctx, run, profileLabels...)
	m.record(work, time.Since(start), err != nil)
	return err
}

func (m *WorkMetrics) record(work string, wallTime time.Duration, failed bool) {
	value, _ := m.entries.LoadOrStore(work, &workMetricsEntry{
		metrics: workMetrics{work: work},
	})
	entry := value.(*workMetricsEntry)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.metrics.calls++
	if failed {
		entry.metrics.failures++
	}
	entry.metrics.wallTime += wallTime
	entry.metrics.maxTime = max(entry.metrics.maxTime, wallTime)
}

func (m *WorkMetrics) collect() []workMetrics {
	var results []workMetrics
	m.entries.Range(func(_, value any) bool {
		entry := value.(*workMetricsEntry)
		entry.mu.Lock()
		results = append(results, entry.metrics)
		entry.mu.Unlock()
		return true
	})
	sort.Slice(results, func(i, j int) bool {
		if results[i].wallTime != results[j].wallTime {
			return results[i].wallTime > results[j].wallTime
		}
		return results[i].work < results[j].work
	})
	return results
}

// Print reports named generator work ordered by cumulative wall time. Times
// from concurrent calls overlap and therefore do not sum to generator time.
func (m *WorkMetrics) Print() {
	results := m.collect()
	if len(results) == 0 {
		return
	}
	var output strings.Builder
	w := tabwriter.NewWriter(&output, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "RANK\tWORK\tWALL TIME\tMAX TIME\tCALLS\tFAILURES")
	for i, result := range results {
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%d\t%d\n",
			i+1, result.work, result.wallTime, result.maxTime, result.calls, result.failures)
	}
	_ = w.Flush()
	log.Infof("generator work metrics, cumulative wall time first (concurrent calls overlap):")
	log.Info(strings.TrimRight(output.String(), "\n"))
}
