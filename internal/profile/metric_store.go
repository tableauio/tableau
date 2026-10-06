package profile

import "sync"

// metricStore provides the synchronized keyed accumulation used by both
// generator-operation and sheet-parser metrics.
type metricStore[K comparable, V any] struct {
	entries sync.Map // K -> *metricEntry[V]
}

type metricEntry[V any] struct {
	mu    sync.Mutex
	value V
}

func (s *metricStore[K, V]) update(key K, initial V, apply func(*V)) {
	stored, _ := s.entries.LoadOrStore(key, &metricEntry[V]{value: initial})
	entry := stored.(*metricEntry[V])
	entry.mu.Lock()
	apply(&entry.value)
	entry.mu.Unlock()
}

func (s *metricStore[K, V]) each(apply func(*V)) {
	s.entries.Range(func(_, stored any) bool {
		entry := stored.(*metricEntry[V])
		entry.mu.Lock()
		apply(&entry.value)
		entry.mu.Unlock()
		return true
	})
}

func (s *metricStore[K, V]) snapshot() []V {
	var values []V
	s.each(func(value *V) { values = append(values, *value) })
	return values
}

func (s *metricStore[K, V]) clear() {
	s.entries.Clear()
}
