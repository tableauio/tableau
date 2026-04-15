package xerrors

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

// Collector accumulates errors concurrently up to a configurable limit.
// Collectors form a hierarchy via [Collector.NewChild]; [Collector.Join]
// recursively merges own errors with children's.
//
// When a child's [collected] error is wrapped (e.g. via [WrapKV]), the
// outer [withMessage] is remembered and re-applied in [Collector.Join],
// producing: collected → withMessage{fields} → joinError{…}.
type Collector struct {
	mu       sync.Mutex
	errs     []error
	children []*Collector
	counter  atomic.Int32
	maxErrs  int32
	parent   *Collector
	outerWM  *withMessage // outer WrapKV layer, re-applied in Join()
}

// NewCollector creates a root Collector.
// maxErrs: <=0 unlimited, 1 fail-fast, >1 stops after N errors.
func NewCollector(maxErrs int) *Collector {
	return &Collector{maxErrs: normalizeMax(maxErrs)}
}

// NewChild creates a child collector registered under the receiver.
// maxErrs semantics are the same as [NewCollector].
func (c *Collector) NewChild(maxErrs int) *Collector {
	child := &Collector{
		maxErrs: normalizeMax(maxErrs),
		parent:  c,
	}
	c.mu.Lock()
	c.children = append(c.children, child)
	c.mu.Unlock()
	return child
}

// normalizeMax converts user-facing maxErrs to internal representation.
func normalizeMax(maxErrs int) int32 {
	if maxErrs <= 0 {
		return math.MaxInt32 // unlimited
	}
	return int32(maxErrs)
}

// Collect stores err and returns the joined error tree if any collector
// in the ancestor chain is full (nil otherwise).
func (c *Collector) Collect(err error) error {
	if err == nil {
		return nil
	}

	// Already-collected error (from a child's Join): remember the outer
	// WrapKV layer on the child collector for re-wrapping in Join().
	var ce *collected
	if errors.As(err, &ce) {
		if ce.origin != nil {
			if wm, ok := err.(*withMessage); ok {
				ce.origin.mu.Lock()
				ce.origin.outerWM = wm
				ce.origin.mu.Unlock()
			}
		}
		if c.IsFull() {
			return c.Join()
		}
		return nil
	}

	anyFull := false
	for cur := c; cur != nil; cur = cur.parent {
		if cur.counter.Add(1) >= cur.maxErrs {
			anyFull = true
		}
	}

	// Store only if no ancestor has exceeded its limit.
	store := true
	for cur := c; cur != nil; cur = cur.parent {
		if cur.counter.Load() > cur.maxErrs {
			store = false
			break
		}
	}
	if store {
		c.mu.Lock()
		c.errs = append(c.errs, err)
		c.mu.Unlock()
	}

	if anyFull {
		return c.Join()
	}
	return nil
}

// IsFull reports whether this collector or any ancestor has reached its limit.
func (c *Collector) IsFull() bool {
	for cur := c; cur != nil; cur = cur.parent {
		if cur.counter.Load() >= cur.maxErrs {
			return true
		}
	}
	return false
}

// HasErrors reports whether this collector's subtree has any errors.
// It is a fast, lock-free check suitable for guarding expensive [Collector.Join] calls.
func (c *Collector) HasErrors() bool {
	return c.counter.Load() > 0
}

// Join returns all errors in this collector's subtree as a single error.
func (c *Collector) Join() error {
	c.mu.Lock()
	ownErrs := make([]error, len(c.errs))
	copy(ownErrs, c.errs)
	kids := make([]*Collector, len(c.children))
	copy(kids, c.children)
	outerWM := c.outerWM
	c.mu.Unlock()

	var nonNil []error
	for _, e := range ownErrs {
		if e != nil {
			nonNil = append(nonNil, e)
		}
	}
	for _, kid := range kids {
		if joined := kid.Join(); joined != nil {
			nonNil = append(nonNil, joined)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	var inner error = &joinError{errs: nonNil, stack: callers(1)}
	// Re-wrap with outer WrapKV fields if present.
	if outerWM != nil {
		inner = &withMessage{cause: inner, fields: outerWM.fields}
	}
	return &collected{
		error:  inner,
		origin: c,
	}
}

// collected marks an error as already joined. Delegates to the inner error.
type collected struct {
	error
	origin *Collector // back-reference to the Collector that created this marker
}

func (c *collected) Error() string { return c.error.Error() }
func (c *collected) Unwrap() error { return c.error }
func (c *collected) Format(s fmt.State, verb rune) {
	if f, ok := c.error.(fmt.Formatter); ok {
		f.Format(s, verb)
	} else {
		_, _ = fmt.Fprintf(s, "%"+string(verb), c.error)
	}
}

// Group ties an [errgroup.Group] to a Collector for concurrent collection.
// Context is cancelled when the collector becomes full.
type Group struct {
	eg        *errgroup.Group
	ctx       context.Context
	cancel    context.CancelFunc
	collector *Collector
}

// NewGroup returns a new Group backed by this Collector.
func (c *Collector) NewGroup(ctx context.Context) *Group {
	ctx, cancel := context.WithCancel(ctx)
	eg, gctx := errgroup.WithContext(ctx)
	return &Group{eg: eg, ctx: gctx, cancel: cancel, collector: c}
}

// Go runs fn in a goroutine and collects its error.
func (g *Group) Go(fn func(ctx context.Context) error) {
	g.eg.Go(func() error {
		if g.collector.Collect(fn(g.ctx)) != nil {
			g.cancel()
		}
		return nil
	})
}

// Wait blocks until all goroutines finish and returns the joined errors.
func (g *Group) Wait() error {
	defer g.cancel()
	_ = g.eg.Wait()
	return g.collector.Join()
}
