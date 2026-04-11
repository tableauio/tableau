package xerrors

import (
	"math"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

// Collector collects errors concurrently up to a maximum count.
// Call IsFull to check for fail-fast; safe for concurrent use.
type Collector struct {
	mu      sync.Mutex
	errs    []error
	counter atomic.Int32
	maxErrs int32
	once    sync.Once // ensures the joined error is computed exactly once
	cached  error     // cached joined error, set once when full
}

// NewCollector creates a Collector with the given max error count.
//   - maxErrs <= 0: unlimited.
//   - maxErrs == 1: fail-fast on first error (default).
//   - maxErrs > 1: stop after N errors.
func NewCollector(maxErrs int) *Collector {
	max := int32(maxErrs)
	if max <= 0 {
		max = math.MaxInt32 // unlimited
	}
	return &Collector{maxErrs: max}
}

// Collect appends err to the collector.
// Returns (false, nil) if err is nil or successfully collected and not yet full.
// Returns (true, joined) once the max count is reached, where joined is all
// collected errors via errors.Join; callers should stop processing immediately.
func (c *Collector) Collect(err error) (bool, error) {
	if err == nil {
		return false, nil
	}
	n := c.counter.Add(1)
	if n > c.maxErrs {
		// Already full: block until the joined error is cached, then return it.
		c.once.Do(func() { c.cached = c.Join() })
		return true, c.cached
	}
	c.mu.Lock()
	c.errs = append(c.errs, err)
	c.mu.Unlock()
	if n == c.maxErrs {
		// Just reached the limit: compute and cache the joined error.
		c.once.Do(func() { c.cached = c.Join() })
		return true, c.cached
	}
	return false, nil
}

// IsFull reports whether the max error count has been reached.
func (c *Collector) IsFull() bool {
	return c.counter.Load() >= c.maxErrs
}

// Join returns all collected errors joined as a structured joinError (with
// stack), or nil if no errors were collected.
func (c *Collector) Join() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var nonNil []error
	for _, e := range c.errs {
		if e != nil {
			nonNil = append(nonNil, e)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	return &joinError{errs: nonNil, stack: callers(1)}
}

// Group ties a fresh errgroup.Group to a Collector for one concurrent batch.
// Use NewGroup per batch so batches run independently.
type Group struct {
	eg        errgroup.Group
	collector *Collector
	waitFull  bool
}

// NewGroup returns a new Group backed by this Collector.
// If waitFull is false, Wait returns the final joined error even when the collector is not full
// If waitFull is true, Wait returns nil when the collector is not full.
func (c *Collector) NewGroup(waitFull bool) *Group {
	return &Group{collector: c, waitFull: waitFull}
}

// Go runs fn in a new goroutine and collects its error.
// If the collector becomes full, the joined error is forwarded to the errgroup.
func (g *Group) Go(fn func() error) {
	g.eg.Go(func() error {
		full, err := g.collector.Collect(fn())
		if full {
			return err
		}
		return nil
	})
}

// Wait blocks until all goroutines finish.
// If the collector is full, it returns the joined error immediately via the errgroup.
// If waitFull is false and the collector is not full, it returns the final joined error of all collected errors.
// If waitFull is true and the collector is not full, it returns nil.
func (g *Group) Wait() error {
	if err := g.eg.Wait(); err != nil {
		return err
	} else if !g.waitFull {
		return g.collector.Join()
	}
	return nil
}
