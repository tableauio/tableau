package xerrors

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Root collector
// ---------------------------------------------------------------------------

func TestCollector_NilError(t *testing.T) {
	c := NewCollector(10)
	assert.NoError(t, c.Collect(nil))
	assert.NoError(t, c.Join())
}

func TestCollector_SingleError(t *testing.T) {
	c := NewCollector(10)
	want := errors.New("single error")
	assert.NoError(t, c.Collect(want))
	assert.ErrorIs(t, c.Join(), want)
}

func TestCollector_MaxErrors(t *testing.T) {
	const max = 3
	c := NewCollector(max)

	for i := range 5 {
		err := c.Collect(fmt.Errorf("error %d", i))
		if i < max-1 {
			assert.NoError(t, err, "error %d: should not be full yet", i)
		} else {
			assert.Error(t, err, "error %d: should be full", i)
		}
	}

	// Only max errors are stored (overflow is counted but not stored).
	assert.Equal(t, max, countJoinedErrors(c.Join()))
}

func TestCollector_Concurrent(t *testing.T) {
	const max = 10
	c := NewCollector(max)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Collect(fmt.Errorf("error %d", i))
		}()
	}
	wg.Wait()

	assert.Equal(t, max, countJoinedErrors(c.Join()))
}

func TestCollector_JoinNilWhenEmpty(t *testing.T) {
	assert.NoError(t, NewCollector(5).Join())
}

func TestCollector_IsFull(t *testing.T) {
	const max = 3
	c := NewCollector(max)
	assert.False(t, c.IsFull())

	for i := range max - 1 {
		c.Collect(fmt.Errorf("error %d", i))
		assert.False(t, c.IsFull())
	}

	assert.Error(t, c.Collect(fmt.Errorf("last")))
	assert.True(t, c.IsFull())

	// Overflow: still full.
	assert.Error(t, c.Collect(fmt.Errorf("overflow")))
	assert.True(t, c.IsFull())
}

func TestCollector_Unlimited(t *testing.T) {
	c := NewCollector(0)
	for i := range 100 {
		assert.NoError(t, c.Collect(fmt.Errorf("error %d", i)))
	}
	assert.False(t, c.IsFull())
	assert.Equal(t, 100, countJoinedErrors(c.Join()))
}

func TestCollector_FailFast(t *testing.T) {
	c := NewCollector(1)
	assert.Error(t, c.Collect(fmt.Errorf("boom")))
	assert.True(t, c.IsFull())
}

// ---------------------------------------------------------------------------
// Parent-child
// ---------------------------------------------------------------------------

// Child.Collect increments counters on self, parent, and root.
func TestChild_CountPropagation(t *testing.T) {
	const max = 3
	root := NewCollector(max)
	child := root.NewChild(0) // unlimited

	for i := range max - 1 {
		assert.NoError(t, child.Collect(fmt.Errorf("err %d", i)))
	}

	assert.Error(t, child.Collect(fmt.Errorf("err %d", max-1)))
	assert.True(t, root.IsFull())
	assert.Equal(t, max, countJoinedErrors(child.Join()))
}

// Child with its own maxErrs becomes full independently of root.
func TestChild_OwnLimit(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(2)

	child.Collect(fmt.Errorf("err 1"))
	assert.False(t, child.IsFull())

	assert.Error(t, child.Collect(fmt.Errorf("err 2")))
	assert.True(t, child.IsFull())
	assert.False(t, root.IsFull(), "root should NOT be full (2 < 10)")
}

// Child with maxErrs=1 is fail-fast at its own level.
func TestChild_FailFast(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(1)

	assert.Error(t, child.Collect(fmt.Errorf("first")))
	assert.True(t, child.IsFull())
	assert.False(t, root.IsFull())
}

// IsFull on a child returns true when a parent is full.
func TestChild_IsFullRespectsAncestors(t *testing.T) {
	root := NewCollector(2)
	child := root.NewChild(0) // unlimited

	root.Collect(fmt.Errorf("root err 1"))
	root.Collect(fmt.Errorf("root err 2"))

	assert.True(t, root.IsFull())
	assert.True(t, child.IsFull(), "child should be full because root is full")
}

// Multiple children share the root's counter.
func TestChild_MultipleShareRootCounter(t *testing.T) {
	root := NewCollector(4)
	c1 := root.NewChild(0)
	c2 := root.NewChild(0)

	c1.Collect(fmt.Errorf("c1 err 1"))
	c1.Collect(fmt.Errorf("c1 err 2"))
	assert.False(t, root.IsFull())

	c2.Collect(fmt.Errorf("c2 err 1"))
	assert.Error(t, c2.Collect(fmt.Errorf("c2 err 2")))
	assert.True(t, root.IsFull())

	assert.Equal(t, 2, countJoinedErrors(c1.Join()))
	assert.Equal(t, 2, countJoinedErrors(c2.Join()))
}

// child.Collect(nil) is a no-op.
func TestChild_NilError(t *testing.T) {
	root := NewCollector(3)
	child := root.NewChild(0)

	assert.NoError(t, child.Collect(nil))
	assert.False(t, root.IsFull())
	assert.NoError(t, child.Join())
}

// Concurrent child collectors are thread-safe.
func TestChild_Concurrent(t *testing.T) {
	const max = 20
	root := NewCollector(max)

	var wg sync.WaitGroup
	for g := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			child := root.NewChild(0)
			for i := range 5 {
				child.Collect(fmt.Errorf("g%d err %d", g, i))
			}
		}()
	}
	wg.Wait()

	assert.True(t, root.IsFull())
}

// ---------------------------------------------------------------------------
// Tree hierarchy
// ---------------------------------------------------------------------------

// Join recursively includes children's errors.
func TestTree_JoinIncludesChildren(t *testing.T) {
	root := NewCollector(0)
	c1 := root.NewChild(0)
	c2 := root.NewChild(0)

	c1.Collect(fmt.Errorf("c1 err"))
	c2.Collect(fmt.Errorf("c2 err"))

	assert.Equal(t, 2, countJoinedErrors(root.Join()))
}

// Grandchild.Collect increments grandchild, child, AND root.
func TestTree_GrandchildCountPropagation(t *testing.T) {
	const max = 3
	root := NewCollector(max)
	child := root.NewChild(0)
	gc := child.NewChild(0)

	for i := range max - 1 {
		assert.NoError(t, gc.Collect(fmt.Errorf("gc err %d", i)))
	}
	assert.Error(t, gc.Collect(fmt.Errorf("gc err %d", max-1)))

	// All ancestors are full because root is full.
	assert.True(t, root.IsFull())
	assert.True(t, child.IsFull())
	assert.True(t, gc.IsFull())

	// Join tree: gc has 3 errors, child wraps gc, root wraps child.
	assert.Equal(t, max, countJoinedErrors(gc.Join()))
	assert.Equal(t, 1, countJoinedErrors(child.Join()))
	assert.Equal(t, 1, countJoinedErrors(root.Join()))
}

// Child becomes full before root (child.maxErrs < root.maxErrs).
func TestTree_ChildFullBeforeRoot(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(3)

	child.Collect(fmt.Errorf("err 1"))
	child.Collect(fmt.Errorf("err 2"))
	assert.False(t, child.IsFull())

	assert.Error(t, child.Collect(fmt.Errorf("err 3")))
	assert.True(t, child.IsFull())
	assert.False(t, root.IsFull(), "root should NOT be full (3 < 10)")
}

// Root becomes full before children (root.maxErrs < child.maxErrs).
func TestTree_RootFullBeforeChildren(t *testing.T) {
	root := NewCollector(3)
	c1 := root.NewChild(5)
	c2 := root.NewChild(5)

	c1.Collect(fmt.Errorf("c1 err 1"))
	c1.Collect(fmt.Errorf("c1 err 2"))
	assert.False(t, root.IsFull())

	assert.Error(t, c2.Collect(fmt.Errorf("c2 err 1")))
	assert.True(t, root.IsFull())
	// Both children report full because root is full.
	assert.True(t, c1.IsFull())
	assert.True(t, c2.IsFull())
}

// IsFull walks UP (ancestors), not DOWN (children).
func TestTree_IsFullDoesNotWalkDown(t *testing.T) {
	root := NewCollector(100)
	child := root.NewChild(10)
	gc := child.NewChild(2)

	gc.Collect(fmt.Errorf("err 1"))
	gc.Collect(fmt.Errorf("err 2"))
	assert.True(t, gc.IsFull(), "grandchild should be full (2/2)")

	// Parent and root are NOT full — IsFull only walks up.
	assert.False(t, child.IsFull(), "child should not be full (2/10, ancestors not full)")
	assert.False(t, root.IsFull(), "root should not be full (2/100)")
}

// 4-level deep tree with per-level limits.
func TestTree_DeepHierarchy(t *testing.T) {
	root := NewCollector(100)
	l1 := root.NewChild(50)
	l2 := l1.NewChild(10)
	l3 := l2.NewChild(3)

	for i := range 3 {
		l3.Collect(fmt.Errorf("deep err %d", i))
	}
	assert.True(t, l3.IsFull(), "l3 full (3/3)")
	assert.False(t, l2.IsFull(), "l2 not full (3/10, ancestors not full)")
	assert.False(t, l1.IsFull(), "l1 not full (3/50)")
	assert.False(t, root.IsFull(), "root not full (3/100)")

	assert.NotNil(t, root.Join())
}

// Mid-level limit stops its subtree while siblings continue.
func TestTree_MidLevelLimitStopsSubtree(t *testing.T) {
	root := NewCollector(100)
	left := root.NewChild(2)  // tight limit
	right := root.NewChild(0) // unlimited

	left.Collect(fmt.Errorf("left 1"))
	assert.Error(t, left.Collect(fmt.Errorf("left 2")))
	assert.True(t, left.IsFull())

	// Right sibling is unaffected.
	assert.False(t, right.IsFull())
	assert.NoError(t, right.Collect(fmt.Errorf("right 1")))
}

// Multiple grandchildren under different children all share root counter.
func TestTree_GrandchildrenShareRootCounter(t *testing.T) {
	root := NewCollector(4)
	c1 := root.NewChild(0)
	c2 := root.NewChild(0)
	gc1 := c1.NewChild(0)
	gc2 := c2.NewChild(0)

	gc1.Collect(fmt.Errorf("gc1 err 1"))
	gc1.Collect(fmt.Errorf("gc1 err 2"))
	gc2.Collect(fmt.Errorf("gc2 err 1"))
	assert.Error(t, gc2.Collect(fmt.Errorf("gc2 err 2")), "4th error should hit root limit")
	assert.True(t, root.IsFull())
}

// Join on a mid-level node returns only its subtree, not siblings.
func TestTree_JoinReturnsOnlySubtree(t *testing.T) {
	root := NewCollector(0)
	c1 := root.NewChild(0)
	c2 := root.NewChild(0)

	c1.Collect(fmt.Errorf("c1 err"))
	c2.Collect(fmt.Errorf("c2 err"))

	// c1.Join() should only contain c1's error, not c2's.
	assert.Equal(t, 1, countJoinedErrors(c1.Join()))
	assert.Equal(t, 1, countJoinedErrors(c2.Join()))
	// root.Join() contains both.
	assert.Equal(t, 2, countJoinedErrors(root.Join()))
}

// Simulates the table parser pattern: root → rowChild → fieldChild.
func TestTree_ParserPattern(t *testing.T) {
	root := NewCollector(10)
	rowChild := root.NewChild(0)

	for row := range 2 {
		fc := rowChild.NewChild(0)
		for f := range 2 {
			fc.Collect(fmt.Errorf("row%d field%d err", row, f))
		}
	}

	assert.False(t, root.IsFull(), "4 < 10")
	assert.Equal(t, 2, countJoinedErrors(rowChild.Join()), "2 field children")
	assert.Equal(t, 1, countJoinedErrors(root.Join()), "1 rowChild join")
}

// No double-counting: each error is Collect'd once at the leaf.
func TestTree_NoDoubleCount(t *testing.T) {
	root := NewCollector(3)
	row := root.NewChild(0)

	fc1 := row.NewChild(0)
	fc1.Collect(fmt.Errorf("r1 f1"))
	fc1.Collect(fmt.Errorf("r1 f2"))

	fc2 := row.NewChild(0)
	assert.Error(t, fc2.Collect(fmt.Errorf("r2 f1")), "3rd error hits root limit")
	assert.True(t, root.IsFull())
}

// Join returns nil when all children are empty.
func TestTree_JoinEmptyChildren(t *testing.T) {
	root := NewCollector(10)
	_ = root.NewChild(0)
	_ = root.NewChild(0)
	assert.NoError(t, root.Join())
}

// Join includes both own errors and children's errors.
func TestTree_MixedOwnAndChildErrors(t *testing.T) {
	root := NewCollector(0)
	root.Collect(fmt.Errorf("root own"))

	child := root.NewChild(0)
	child.Collect(fmt.Errorf("child err"))

	assert.Equal(t, 2, countJoinedErrors(root.Join()))
}

// Overflow: errors beyond the collector's own limit are counted but not stored.
func TestTree_OverflowNotStored(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(2) // child stores at most 2

	child.Collect(fmt.Errorf("err 1"))
	child.Collect(fmt.Errorf("err 2"))
	child.Collect(fmt.Errorf("err 3 (overflow)"))
	child.Collect(fmt.Errorf("err 4 (overflow)"))

	// Only 2 errors stored (child's own limit), even though 4 were collected.
	assert.Equal(t, 2, countJoinedErrors(child.Join()))
	// Root counter reflects all 4.
	assert.False(t, root.IsFull(), "root should not be full (4 < 10)")
}

// ---------------------------------------------------------------------------
// Group (NewGroup / Go / Wait)
// ---------------------------------------------------------------------------

// Go collects errors from goroutines; Wait returns joined error.
func TestGroup_BasicCollect(t *testing.T) {
	c := NewCollector(10)
	g := c.NewGroup(context.Background())

	for i := range 3 {
		i := i
		g.Go(func(ctx context.Context) error { return fmt.Errorf("err %d", i) })
	}

	err := g.Wait()
	assert.Error(t, err)
	assert.Equal(t, 3, countJoinedErrors(err))
}

// Go with nil-returning goroutines; Wait returns nil.
func TestGroup_NilErrors(t *testing.T) {
	c := NewCollector(10)
	g := c.NewGroup(context.Background())

	for range 3 {
		g.Go(func(ctx context.Context) error { return nil })
	}

	assert.NoError(t, g.Wait())
}

// Collector not full: Wait still returns Join() of collected errors.
func TestGroup_NotFull(t *testing.T) {
	c := NewCollector(10)
	g := c.NewGroup(context.Background())

	g.Go(func(ctx context.Context) error { return fmt.Errorf("err 1") })
	g.Go(func(ctx context.Context) error { return fmt.Errorf("err 2") })

	err := g.Wait()
	assert.Error(t, err, "Wait should return Join() even when not full")
	assert.Equal(t, 2, countJoinedErrors(err))
}

// Collector becomes full: Wait returns the joined error.
func TestGroup_Full(t *testing.T) {
	c := NewCollector(2)
	g := c.NewGroup(context.Background())

	g.Go(func(ctx context.Context) error { return fmt.Errorf("err 1") })
	g.Go(func(ctx context.Context) error { return fmt.Errorf("err 2") })

	err := g.Wait()
	assert.Error(t, err, "Wait should return error when collector is full")
}

// Collector becomes full mid-flight with concurrent goroutines.
func TestGroup_ConcurrentFull(t *testing.T) {
	const max = 5
	c := NewCollector(max)
	g := c.NewGroup(context.Background())

	for i := range 20 {
		i := i
		g.Go(func(ctx context.Context) error { return fmt.Errorf("err %d", i) })
	}

	err := g.Wait()
	assert.Error(t, err, "should return error when collector becomes full")
	assert.True(t, c.IsFull())
}

// Group backed by a child collector in a hierarchy.
func TestGroup_WithChildCollector(t *testing.T) {
	root := NewCollector(5)
	child := root.NewChild(0)
	g := child.NewGroup(context.Background())

	for i := range 3 {
		i := i
		g.Go(func(ctx context.Context) error { return fmt.Errorf("child err %d", i) })
	}

	err := g.Wait()
	assert.Error(t, err)
	assert.Equal(t, 3, countJoinedErrors(err))
	// Root counter should reflect child's errors.
	assert.False(t, root.IsFull(), "root should not be full (3 < 5)")
}

// Group backed by a child; root becomes full via child's Group.
func TestGroup_ChildFullHitsRoot(t *testing.T) {
	root := NewCollector(3)
	child := root.NewChild(0)
	g := child.NewGroup(context.Background())

	for i := range 5 {
		i := i
		g.Go(func(ctx context.Context) error { return fmt.Errorf("err %d", i) })
	}

	err := g.Wait()
	assert.Error(t, err, "root should become full via child's Group")
	assert.True(t, root.IsFull())
}

// Multiple Groups on the same collector run independently.
func TestGroup_MultipleBatches(t *testing.T) {
	c := NewCollector(10)

	g1 := c.NewGroup(context.Background())
	g1.Go(func(ctx context.Context) error { return fmt.Errorf("batch1 err") })
	err1 := g1.Wait()
	assert.Error(t, err1)
	assert.Equal(t, 1, countJoinedErrors(err1))

	g2 := c.NewGroup(context.Background())
	g2.Go(func(ctx context.Context) error { return fmt.Errorf("batch2 err") })
	err2 := g2.Wait()
	assert.Error(t, err2)
	// g2.Wait() calls c.Join() which includes all 2 errors accumulated so far.
	assert.Equal(t, 2, countJoinedErrors(err2))
}

// Context is cancelled when collector becomes full.
func TestGroup_ContextCancelled(t *testing.T) {
	c := NewCollector(1) // fail-fast
	g := c.NewGroup(context.Background())

	started := make(chan struct{})
	g.Go(func(ctx context.Context) error {
		close(started)
		// Block until context is cancelled by the other goroutine.
		<-ctx.Done()
		return nil
	})
	<-started // ensure the first goroutine is running

	g.Go(func(ctx context.Context) error {
		return fmt.Errorf("boom") // triggers full → cancels context
	})

	err := g.Wait()
	assert.Error(t, err)
	assert.True(t, c.IsFull())
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func countJoinedErrors(err error) int {
	// Unwrap collected marker if present.
	var ce *collected
	if errors.As(err, &ce) {
		err = ce.error
	}
	type joinError interface {
		Unwrap() []error
	}
	if je, ok := err.(joinError); ok {
		return len(je.Unwrap())
	}
	if err != nil {
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------------
// Collected marker
// ---------------------------------------------------------------------------

// Join returns a collected-wrapped error; Collect skips it.
func TestCollected_JoinReturnsCollectedMarker(t *testing.T) {
	c := NewCollector(10)
	c.Collect(fmt.Errorf("err 1"))

	joined := c.Join()
	assert.Error(t, joined)

	var ce *collected
	assert.True(t, errors.As(joined, &ce), "Join() should return a collected-wrapped error")
}

// Collect skips collected-marked errors.
func TestCollected_CollectSkipsCollectedError(t *testing.T) {
	c := NewCollector(10)
	c.Collect(fmt.Errorf("err 1"))

	joined := c.Join()
	assert.Error(t, joined)

	// Create a new collector and Collect the joined error.
	// The collected marker should cause it to be skipped.
	c2 := NewCollector(10)
	c2.Collect(joined)
	assert.NoError(t, c2.Join(), "collected error should be skipped")
}

// Collect skips collected marker even through WrapKV.
func TestCollected_CollectSkipsWrappedCollectedError(t *testing.T) {
	c := NewCollector(10)
	c.Collect(fmt.Errorf("err 1"))

	joined := c.Join()
	wrapped := WrapKV(joined, KeyBookName, "test.xlsx")

	c2 := NewCollector(10)
	c2.Collect(wrapped) // collected marker detected, skip entirely
	assert.NoError(t, c2.Join(), "WrapKV'd collected error should be skipped")
}

// collected marker is transparent: Error() delegates to inner.
func TestCollected_ErrorDelegates(t *testing.T) {
	c := NewCollector(10)
	c.Collect(fmt.Errorf("hello"))

	joined := c.Join()
	assert.Contains(t, joined.Error(), "hello")
}

// collected marker is transparent: errors.Is works through it.
func TestCollected_ErrorsIsWorksThrough(t *testing.T) {
	target := fmt.Errorf("target")
	c := NewCollector(10)
	c.Collect(target)

	joined := c.Join()
	assert.True(t, errors.Is(joined, target), "errors.Is should see through collected marker")
}

// Simulates recursive parseMessage: inner fieldChild.Join() flows to outer
// fieldChild.Collect(), which skips it. Parent's Join() includes both via
// tree auto-join.
//
//   root
//   └── docChild
//       ├── outerChild  (skips innerJoined)
//       └── innerChild  (has "inner err 1", "inner err 2")
func TestCollected_CrossSubtreeCollection(t *testing.T) {
	root := NewCollector(10)
	docChild := root.NewChild(0)
	outerChild := docChild.NewChild(0)
	innerChild := docChild.NewChild(0)

	innerChild.Collect(fmt.Errorf("inner err 1"))
	innerChild.Collect(fmt.Errorf("inner err 2"))

	innerJoined := innerChild.Join()
	assert.Error(t, innerJoined)

	// Outer Collect skips it (collected marker).
	outerChild.Collect(innerJoined)

	// outerChild has no own errors.
	assert.NoError(t, outerChild.Join())

	// docChild.Join() includes both via tree auto-join.
	docJoined := docChild.Join()
	assert.Error(t, docJoined)
	assert.Contains(t, docJoined.Error(), "inner err 1")
	assert.Contains(t, docJoined.Error(), "inner err 2")
}

// Tree auto-join: child errors appear in root.Join() via tree traversal.
func TestCollected_TreeAutoJoinStillWorks(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0)

	child.Collect(fmt.Errorf("err 1"))
	child.Collect(fmt.Errorf("err 2"))

	// root has no own errors, but child has 2.
	rootJoined := root.Join()
	assert.Error(t, rootJoined)
	// 1 child join (containing 2 inner errors)
	assert.Equal(t, 1, countJoinedErrors(rootJoined))
}

// ---------------------------------------------------------------------------
// HasErrors (fast path guard for Join)
// ---------------------------------------------------------------------------

func TestHasErrors_EmptyCollector(t *testing.T) {
	c := NewCollector(10)
	assert.False(t, c.HasErrors())
}

func TestHasErrors_WithOwnErrors(t *testing.T) {
	c := NewCollector(10)
	c.Collect(fmt.Errorf("err"))
	assert.True(t, c.HasErrors())
}

func TestHasErrors_EmptyWithEmptyChildren(t *testing.T) {
	root := NewCollector(10)
	_ = root.NewChild(0)
	_ = root.NewChild(0)
	assert.False(t, root.HasErrors())
}

// HasErrors detects errors in children via counter propagation.
func TestHasErrors_ErrorInChild(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0)
	child.Collect(fmt.Errorf("child err"))

	assert.True(t, root.HasErrors())
}

// HasErrors detects errors in grandchildren.
func TestHasErrors_ErrorInGrandchild(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0)
	gc := child.NewChild(0)
	gc.Collect(fmt.Errorf("deep err"))

	assert.True(t, root.HasErrors())
	assert.True(t, child.HasErrors())
	assert.True(t, gc.HasErrors())
}
