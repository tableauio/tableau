package xerrors

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

// Collector collects multiple errors concurrently up to a maximum count.
// When the maximum is reached it cancels an internal context so that all
// goroutines sharing that context can detect the situation via Done() and
// exit immediately (fail-fast).
// It is safe for concurrent use.
type Collector struct {
	mu      sync.Mutex
	errs    []error
	counter atomic.Int32
	maxErrs int32

	cancel context.CancelFunc
	done   <-chan struct{}
}

// NewCollector creates a new Collector with the given maximum error count.
// The returned Collector's Done() channel is derived from ctx: it is closed
// either when ctx itself is cancelled or when the maximum error count is
// reached, whichever comes first.
func NewCollector(ctx context.Context, maxErrs int) *Collector {
	childCtx, cancel := context.WithCancel(ctx)
	return &Collector{
		maxErrs: int32(maxErrs),
		cancel:  cancel,
		done:    childCtx.Done(),
	}
}

// Add tries to add an error to the collector. It returns true if the error was
// accepted (i.e., the collector has not yet reached its maximum). Nil errors
// are silently ignored and return false.
// When the maximum is reached the internal context is cancelled so that all
// goroutines watching Done() can exit immediately.
func (c *Collector) Add(err error) bool {
	if err == nil {
		return false
	}
	n := c.counter.Add(1)
	if n > c.maxErrs {
		return false
	}
	c.mu.Lock()
	c.errs = append(c.errs, err)
	c.mu.Unlock()
	if n == c.maxErrs {
		// Reached the limit — signal all goroutines to stop.
		c.cancel()
	}
	return true
}

// IsFull reports whether the collector has reached its maximum error count.
// Callers can use this to fail-fast and avoid launching new goroutines once
// the limit is hit.
func (c *Collector) IsFull() bool {
	return c.counter.Load() >= c.maxErrs
}

// Done returns a channel that is closed when the error limit is reached or
// when the parent context passed to NewCollector is cancelled.
// Goroutines should select on this channel to implement fail-fast behaviour.
func (c *Collector) Done() <-chan struct{} {
	return c.done
}

// Join returns all collected errors joined together via errors.Join, or nil if
// no errors were collected.
func (c *Collector) Join() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return errors.Join(c.errs...)
}


