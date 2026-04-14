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
// Collectors form a hierarchy via [Collector.NewChild]. Collect increments
// counters on self and every ancestor; IsFull checks self and all ancestors;
// Join recursively merges own errors with children's.
//
// An optional set of key-value pairs (kvPairs) can be attached when creating
// a child via [Collector.NewChild]. These pairs are carried by the
// [collected] marker returned from [Collector.Join] (which implements
// [fieldsCarrier]), so [NewDesc] naturally extracts them while walking the
// error chain and propagates them to every leaf error through
// mergeOuterFields. Callers do not need to manually call [WrapKV].
type Collector struct {
	mu       sync.Mutex
	errs     []error
	children []*Collector
	counter  atomic.Int32
	maxErrs  int32
	parent   *Collector
	kvPairs  []any // auto-wrapped onto each collected error
}

// NewCollector creates a root Collector.
// maxErrs <= 0 means unlimited; 1 means fail-fast; >1 stops after N errors.
func NewCollector(maxErrs int) *Collector {
	return &Collector{maxErrs: normalizeMax(maxErrs)}
}

// NewChild creates a child collector with its own limit, registered under
// the receiver. Collect on the child increments counters up to the root.
// maxErrs semantics are the same as [NewCollector].
//
// Optional kvPairs are carried by the [collected] marker returned from
// [Collector.Join] on the returned child. Because [collected] implements
// [fieldsCarrier], [NewDesc] naturally extracts these fields while walking
// the error chain and propagates them to every leaf error through
// mergeOuterFields. The caller does not need to wrap errors manually.
func (c *Collector) NewChild(maxErrs int, kvPairs ...any) *Collector {
	child := &Collector{
		maxErrs: normalizeMax(maxErrs),
		parent:  c,
		kvPairs: kvPairs,
	}
	c.mu.Lock()
	c.children = append(c.children, child)
	c.mu.Unlock()
	return child
}

// normalizeMax converts user-facing maxErrs to an internal value.
func normalizeMax(maxErrs int) int32 {
	if maxErrs <= 0 {
		return math.MaxInt32 // unlimited
	}
	return int32(maxErrs)
}

// Collect stores err, increments counters up the ancestor chain, and returns
// the joined error tree if any collector is full (nil otherwise).
// Already-joined errors (marked by [Collector.Join]) are skipped.
func (c *Collector) Collect(err error) error {
	if err == nil {
		return nil
	}

	// Skip collected errors — already in the tree.
	var ce *collected
	if errors.As(err, &ce) {
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

	// Store the error only if no collector in the ancestor chain has
	// exceeded its limit. This ensures that the total number of stored
	// errors in any subtree never exceeds the ancestor's limit.
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

// Join returns all errors in this collector's subtree as a single error,
// or nil if empty. The result is marked so [Collector.Collect] skips it.
func (c *Collector) Join() error {
	c.mu.Lock()
	ownErrs := make([]error, len(c.errs))
	copy(ownErrs, c.errs)
	kids := make([]*Collector, len(c.children))
	copy(kids, c.children)
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
	return &collected{
		error:   &joinError{errs: nonNil, stack: callers(1)},
		kvPairs: c.kvPairs,
	}
}

// collected marks an error as already stored in the collector tree.
// Transparent: Error(), Unwrap(), and Format() delegate to the inner error.
//
// It optionally carries key-value pairs inherited from the owning
// [Collector]. Because it implements [fieldsCarrier], [NewDesc] extracts
// these fields while walking the error chain and propagates them to every
// leaf error through mergeOuterFields — no extra [WrapKV] layer needed.
type collected struct {
	error
	kvPairs []any // inherited from Collector; may be nil
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

// Fields implements fieldsCarrier so that [NewDesc] can extract the
// collector-level key-value pairs while traversing the error chain.
func (c *collected) Fields() map[string]any {
	if len(c.kvPairs) == 0 {
		return nil
	}
	m := make(map[string]any, len(c.kvPairs)/2)
	for i := 0; i+1 < len(c.kvPairs); i += 2 {
		if k, ok := c.kvPairs[i].(string); ok {
			m[k] = c.kvPairs[i+1]
		}
	}
	return m
}

// Group ties an [errgroup.Group] to a Collector for concurrent error collection.
// Context is cancelled when the collector becomes full, enabling early exit.
type Group struct {
	eg        *errgroup.Group
	ctx       context.Context
	cancel    context.CancelFunc
	collector *Collector
}

// NewGroup returns a new Group backed by this Collector.
// The provided ctx is used as the parent context for the group's context.
func (c *Collector) NewGroup(ctx context.Context) *Group {
	ctx, cancel := context.WithCancel(ctx)
	eg, gctx := errgroup.WithContext(ctx)
	return &Group{eg: eg, ctx: gctx, cancel: cancel, collector: c}
}

// Go runs fn in a goroutine and collects its error.
// When the collector becomes full, the group's context is cancelled
// to signal other goroutines to exit early.
func (g *Group) Go(fn func(ctx context.Context) error) {
	g.eg.Go(func() error {
		if g.collector.Collect(fn(g.ctx)) != nil {
			g.cancel()
		}
		return nil
	})
}

// Wait blocks until all goroutines finish and returns the joined error tree.
func (g *Group) Wait() error {
	defer g.cancel()
	_ = g.eg.Wait()
	return g.collector.Join()
}
