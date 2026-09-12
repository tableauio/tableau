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
// When a child's [collected] error is wrapped (e.g. via [WrapKV]), the scope
// fields of the wrapper layers are remembered and re-applied in
// [Collector.Join], producing: collected → withMessage{fields} → joinError{…}.
type Collector struct {
	mu          sync.Mutex
	errs        []error
	children    []*Collector
	counter     atomic.Int32
	maxErrs     int32
	parent      *Collector
	outerFields map[string]any // scope fields of the outer wrappers, re-applied in Join()
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

// Collect accumulates err into the collector.
//
//   - nil err: no-op unless this collector (or an ancestor) is already full,
//     in which case the joined error tree is returned immediately.
//   - already-collected err (*collected): records the outer wrapper layers'
//     scope fields for re-wrapping in Join; does not increment any counter.
//   - ordinary err: increments every ancestor's counter, stores the error if
//     it is within every ancestor's budget, and returns the joined error tree
//     if any ancestor has now reached its limit (nil otherwise).
func (c *Collector) Collect(err error) error {
	if err == nil {
		if c.IsFull() {
			return c.Join()
		}
		return nil
	}

	// Already-collected error (from a Join() somewhere): if the collected
	// error originates from the same collector tree as c (shares the same
	// root), it is or will be reachable from the root via tree auto-join, so
	// we must not double-count it here. We only record the outer wrapper
	// layers' scope fields on the originating collector for re-wrapping in
	// its Join(). The assignment is unconditional: a wrapper carrying no
	// scope fields must clear the previously recorded ones instead of
	// leaving them to be re-applied to this unrelated join.
	//
	// Otherwise (foreign collected, e.g. from an unrelated parser-local
	// fail-fast collector that is not part of c's tree), fall through and
	// treat it as an ordinary error so it is still stored and counted on
	// this collector; without this fallback the error would silently vanish.
	var ce *collected
	if errors.As(err, &ce) && ce.origin != nil && ce.origin.sameTreeAs(c) {
		fields := outerScopeFields(err, ce)
		ce.origin.mu.Lock()
		ce.origin.outerFields = fields
		ce.origin.mu.Unlock()
		if c.IsFull() {
			return c.Join()
		}
		return nil
	}

	// Increment all ancestor counters; track whether any hit the limit (anyFull)
	// and whether this error is within every ancestor's budget (store).
	// n == maxErrs is the last accepted slot; n > maxErrs means overflow.
	anyFull, store := false, true
	for cur := c; cur != nil; cur = cur.parent {
		n := cur.counter.Add(1)
		if n >= cur.maxErrs {
			anyFull = true
		}
		if n > cur.maxErrs {
			store = false
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

// root walks the parent chain and returns the topmost collector.
func (c *Collector) root() *Collector {
	cur := c
	for cur.parent != nil {
		cur = cur.parent
	}
	return cur
}

// sameTreeAs reports whether c and other share the same root collector,
// i.e. they belong to the same collector tree built via NewChild.
func (c *Collector) sameTreeAs(other *Collector) bool {
	if c == nil || other == nil {
		return false
	}
	return c.root() == other.root()
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
	outerFields := c.outerFields
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
	// Re-wrap with the recorded outer scope fields if present.
	if len(outerFields) > 0 {
		inner = &withMessage{cause: inner, fields: outerFields}
	}
	return &collected{
		error:  inner,
		origin: c,
	}
}

// outerScopeFields merges the [scopeKeys] fields of every wrapper layer between
// err and the collected marker ce, with inner layers winning on key conflicts.
// Returns nil if no layer carries one.
//
// All intermediate layers must be walked, not just the outermost one: a wrapper
// chain often spreads its fields across several layers (e.g. confgen adds
// Module in one layer and BookName/SheetName in another), so keeping only the
// outermost layer would silently drop the rest.
//
// Non-scope fields are skipped because Join() re-applies these to the whole
// subtree: a cell position or field name belongs to a single error, and
// broadcasting it would point sibling errors at a cell they never touched.
func outerScopeFields(err error, ce *collected) map[string]any {
	var fields map[string]any
	for cur := err; cur != nil && cur != error(ce); cur = errors.Unwrap(cur) {
		fc, ok := cur.(fieldsCarrier)
		if !ok {
			continue
		}
		// Walking outer -> inner, so inner layers win.
		for k, v := range fc.Fields() {
			if !scopeKeys[k] {
				continue
			}
			if fields == nil {
				fields = make(map[string]any)
			}
			fields[k] = v
		}
	}
	return fields
}

// collected marks an error as already joined. Delegates to the inner error.
type collected struct {
	error
	origin *Collector // back-reference to the Collector that created this marker
}

func (c *collected) Error() string { return c.error.Error() }
func (c *collected) Unwrap() error { return c.error }

// renderWithFields implements [fieldsRenderer], so that outer fields (e.g.
// Module, BookName, SheetName added by an enclosing [WrapKV]) are propagated
// into the joined children instead of being dropped. Without this, rendering
// falls back to the inner error's Error(), which loses the outer fields and
// thus renders the default message template rather than the module-specific
// one (e.g. confgen).
func (c *collected) renderWithFields(outerFields map[string]any) string {
	return renderCause(c.error, outerFields)
}
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
	if !g.collector.HasErrors() {
		return nil
	}
	return g.collector.Join()
}
