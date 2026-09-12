package xerrors

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			_ = c.Collect(fmt.Errorf("error %d", i))
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
		_ = c.Collect(fmt.Errorf("error %d", i))
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

	_ = child.Collect(fmt.Errorf("err 1"))
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

	_ = root.Collect(fmt.Errorf("root err 1"))
	_ = root.Collect(fmt.Errorf("root err 2"))

	assert.True(t, root.IsFull())
	assert.True(t, child.IsFull(), "child should be full because root is full")
}

// Multiple children share the root's counter.
func TestChild_MultipleShareRootCounter(t *testing.T) {
	root := NewCollector(4)
	c1 := root.NewChild(0)
	c2 := root.NewChild(0)

	_ = c1.Collect(fmt.Errorf("c1 err 1"))
	_ = c1.Collect(fmt.Errorf("c1 err 2"))
	assert.False(t, root.IsFull())

	_ = c2.Collect(fmt.Errorf("c2 err 1"))
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
				_ = child.Collect(fmt.Errorf("g%d err %d", g, i))
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

	_ = c1.Collect(fmt.Errorf("c1 err"))
	_ = c2.Collect(fmt.Errorf("c2 err"))

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

	_ = child.Collect(fmt.Errorf("err 1"))
	_ = child.Collect(fmt.Errorf("err 2"))
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

	_ = c1.Collect(fmt.Errorf("c1 err 1"))
	_ = c1.Collect(fmt.Errorf("c1 err 2"))
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

	_ = gc.Collect(fmt.Errorf("err 1"))
	_ = gc.Collect(fmt.Errorf("err 2"))
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
		_ = l3.Collect(fmt.Errorf("deep err %d", i))
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

	_ = left.Collect(fmt.Errorf("left 1"))
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

	_ = gc1.Collect(fmt.Errorf("gc1 err 1"))
	_ = gc1.Collect(fmt.Errorf("gc1 err 2"))
	_ = gc2.Collect(fmt.Errorf("gc2 err 1"))
	assert.Error(t, gc2.Collect(fmt.Errorf("gc2 err 2")), "4th error should hit root limit")
	assert.True(t, root.IsFull())
}

// Join on a mid-level node returns only its subtree, not siblings.
func TestTree_JoinReturnsOnlySubtree(t *testing.T) {
	root := NewCollector(0)
	c1 := root.NewChild(0)
	c2 := root.NewChild(0)

	_ = c1.Collect(fmt.Errorf("c1 err"))
	_ = c2.Collect(fmt.Errorf("c2 err"))

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
			_ = fc.Collect(fmt.Errorf("row%d field%d err", row, f))
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
	_ = fc1.Collect(fmt.Errorf("r1 f1"))
	_ = fc1.Collect(fmt.Errorf("r1 f2"))

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
	_ = root.Collect(fmt.Errorf("root own"))

	child := root.NewChild(0)
	_ = child.Collect(fmt.Errorf("child err"))

	assert.Equal(t, 2, countJoinedErrors(root.Join()))
}

// Overflow: errors beyond the collector's own limit are counted but not stored.
func TestTree_OverflowNotStored(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(2) // child stores at most 2

	_ = child.Collect(fmt.Errorf("err 1"))
	_ = child.Collect(fmt.Errorf("err 2"))
	_ = child.Collect(fmt.Errorf("err 3 (overflow)"))
	_ = child.Collect(fmt.Errorf("err 4 (overflow)"))

	// Only 2 errors stored (child's own limit), even though 4 were collected.
	assert.Equal(t, 2, countJoinedErrors(child.Join()))
	// Root counter reflects all 4.
	assert.False(t, root.IsFull(), "root should not be full (4 < 10)")
}

// Parent limit caps child storage: child has a large limit but parent's
// smaller limit prevents storing more errors than the parent allows.
func TestTree_ParentLimitCapsChildStorage(t *testing.T) {
	root := NewCollector(3)
	child := root.NewChild(10) // child limit is large, but root limit is 3

	for i := 1; i <= 5; i++ {
		_ = child.Collect(fmt.Errorf("err %d", i))
	}

	assert.True(t, root.IsFull())
	// Only 3 errors stored (capped by root limit), not 5.
	assert.Equal(t, 3, countJoinedErrors(child.Join()))
}

// Grandparent limit caps grandchild storage across 3 levels.
func TestTree_GrandparentLimitCapsGrandchild(t *testing.T) {
	root := NewCollector(4)
	mid := root.NewChild(10)
	leaf := mid.NewChild(10)

	for i := 1; i <= 6; i++ {
		_ = leaf.Collect(fmt.Errorf("err %d", i))
	}

	assert.True(t, root.IsFull())
	// Only 4 errors stored (capped by root limit).
	assert.Equal(t, 4, countJoinedErrors(leaf.Join()))
}

// Multiple children share parent's storage budget: once the parent limit
// is reached, subsequent children cannot store more errors.
func TestTree_SiblingsShareParentStorageBudget(t *testing.T) {
	root := NewCollector(5)
	c1 := root.NewChild(10)
	c2 := root.NewChild(10)

	// c1 stores 3 errors.
	for i := 1; i <= 3; i++ {
		_ = c1.Collect(fmt.Errorf("c1 err %d", i))
	}
	assert.False(t, root.IsFull())

	// c2 can only store 2 more before root is full.
	for i := 1; i <= 4; i++ {
		_ = c2.Collect(fmt.Errorf("c2 err %d", i))
	}
	assert.True(t, root.IsFull())

	// c1 stored 3, c2 stored only 2 (root budget exhausted).
	assert.Equal(t, 3, countJoinedErrors(c1.Join()))
	assert.Equal(t, 2, countJoinedErrors(c2.Join()))
	// root.Join() = c1.Join() + c2.Join() = 2 children joins.
	assert.Equal(t, 2, countJoinedErrors(root.Join()))
}

// Mid-level limit caps its subtree while root has plenty of budget.
func TestTree_MidLevelLimitCapsSubtree(t *testing.T) {
	root := NewCollector(100)
	mid := root.NewChild(3) // tight mid-level limit
	leaf := mid.NewChild(10)

	for i := 1; i <= 5; i++ {
		_ = leaf.Collect(fmt.Errorf("err %d", i))
	}

	assert.True(t, mid.IsFull())
	assert.False(t, root.IsFull())
	// Only 3 errors stored (capped by mid-level limit).
	assert.Equal(t, 3, countJoinedErrors(leaf.Join()))
}

// 4-level hierarchy: the tightest ancestor limit controls storage.
func TestTree_FourLevelTightestAncestorWins(t *testing.T) {
	root := NewCollector(100)
	l1 := root.NewChild(50)
	l2 := l1.NewChild(4) // tightest limit
	l3 := l2.NewChild(20)

	for i := 1; i <= 8; i++ {
		_ = l3.Collect(fmt.Errorf("err %d", i))
	}

	assert.True(t, l2.IsFull())
	assert.False(t, l1.IsFull())
	assert.False(t, root.IsFull())
	// Only 4 errors stored (capped by l2's limit, the tightest ancestor).
	assert.Equal(t, 4, countJoinedErrors(l3.Join()))
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
// Structured error rendering via Stringify (Collector hierarchy)
// ---------------------------------------------------------------------------

func TestCollector_Stringify_ThreeLevel(t *testing.T) {
	global := NewCollector(10)
	book := global.NewChild(5)
	sheet := book.NewChild(3)

	_ = sheet.Collect(fmt.Errorf("field_a: type mismatch"))
	_ = sheet.Collect(fmt.Errorf("field_b: null value"))
	_ = sheet.Collect(fmt.Errorf("row3: missing key"))
	assert.True(t, sheet.IsFull())

	joined := global.Join()
	require.Error(t, joined)
	got := NewDesc(joined).Stringify(false)
	want := `[1] field_a: type mismatch
[2] field_b: null value
[3] row3: missing key`
	assert.Equal(t, want, got)
}

func TestCollector_Stringify_NewKV(t *testing.T) {
	global := NewCollector(10)
	book := global.NewChild(5)
	sheet := book.NewChild(5)

	_ = sheet.Collect(NewKV("invalid integer value",
		KeyModule, ModuleConf,
		KeyBookName, "Items.xlsx",
		KeySheetName, "ItemConf",
		KeyDataCellPos, "C3",
		KeyDataCell, "abc",
	))

	joined := global.Join()
	require.Error(t, joined)
	got := NewDesc(joined).Stringify(false)
	want := `error[E0004]: unknown error
Workbook: Items.xlsx
Worksheet: ItemConf
DataCellPos: C3
DataCell: abc
Reason: invalid integer value
`
	assert.Equal(t, want, got)
}

func TestCollector_Stringify_WrapKV(t *testing.T) {
	global := NewCollector(10)
	book := global.NewChild(5)
	sheet := book.NewChild(5)

	_ = sheet.Collect(NewKV("field1 error",
		KeyModule, ModuleProto,
		KeyBookName, "Hero.csv",
		KeySheetName, "HeroConf",
		KeyNameCellPos, "B1",
		KeyNameCell, "Attack",
		KeyTypeCellPos, "B2",
		KeyTypeCell, "int32",
	))
	_ = sheet.Collect(NewKV("field2 error",
		KeyModule, ModuleProto,
		KeyBookName, "Hero.csv",
		KeySheetName, "HeroConf",
		KeyNameCellPos, "C1",
		KeyNameCell, "Defense",
		KeyTypeCellPos, "C2",
		KeyTypeCell, "string",
	))

	joined := global.Join()
	require.Error(t, joined)
	got := NewDesc(joined).Stringify(false)
	want := `[1] error[E0004]: unknown error
Workbook: Hero.csv
Worksheet: HeroConf
NameCellPos: B1
NameCell: Attack
TypeCellPos: B2
TypeCell: int32
Reason: field1 error

[2] error[E0004]: unknown error
Workbook: Hero.csv
Worksheet: HeroConf
NameCellPos: C1
NameCell: Defense
TypeCellPos: C2
TypeCell: string
Reason: field2 error
`
	assert.Equal(t, want, got)
}

func TestCollector_Stringify_WrapKV_Ecode(t *testing.T) {
	t.Run("modulconf", func(t *testing.T) {
		global := NewCollector(10)
		book := global.NewChild(5)
		sheet := book.NewChild(3)

		_ = sheet.Collect(WrapKV(E2005("dup_key"),
			KeyModule, ModuleConf,
			KeyBookName, "Items.xlsx",
			KeySheetName, "ItemConf",
			KeyDataCellPos, "B3",
			KeyDataCell, "dup_key",
		))

		joined := global.Join()
		require.Error(t, joined)
		got := NewDesc(joined).Stringify(false)
		want := `error[E2005]: map key not unique
Workbook: Items.xlsx
Worksheet: ItemConf
DataCellPos: B3
DataCell: dup_key
Reason: map key "dup_key" already exists
Help: fix duplicate keys and ensure map key is unique
`
		assert.Equal(t, want, got)
	})

	t.Run("moduleproto", func(t *testing.T) {
		global := NewCollector(10)
		book := global.NewChild(5)
		sheet := book.NewChild(3)

		_ = sheet.Collect(WrapKV(E0003("ID", "A1", "B1"),
			KeyModule, ModuleProto,
			KeyBookName, "Hero.csv",
			KeySheetName, "HeroConf",
			KeyNameCellPos, "A1",
			KeyNameCell, "ID",
			KeyTypeCellPos, "A2",
			KeyTypeCell, "int32",
		))

		joined := global.Join()
		require.Error(t, joined)
		got := NewDesc(joined).Stringify(false)
		want := `error[E0003]: duplicate column name
Workbook: Hero.csv
Worksheet: HeroConf
NameCellPos: A1
NameCell: ID
TypeCellPos: A2
TypeCell: int32
Reason: duplicate column name "ID" in both "A1" and "B1"
Help: rename column name and keep sure it is unique in name row
`
		assert.Equal(t, want, got)
	})
}

func TestCollector_Stringify_MixedErrors(t *testing.T) {
	global := NewCollector(10)
	book := global.NewChild(5)
	sheet := book.NewChild(5)

	_ = sheet.Collect(NewKV("invalid integer",
		KeyModule, ModuleConf,
		KeyBookName, "Items.xlsx",
		KeySheetName, "ItemConf",
		KeyDataCellPos, "C3",
		KeyDataCell, "abc",
	))
	_ = sheet.Collect(WrapKV(E2000("int32", "999999999999", int32(-2147483648), int32(2147483647)),
		KeyModule, ModuleConf,
		KeyBookName, "Items.xlsx",
		KeySheetName, "ItemConf",
		KeyDataCellPos, "D3",
		KeyDataCell, "999999999999",
	))
	_ = sheet.Collect(fmt.Errorf("row5: duplicate key"))

	joined := global.Join()
	require.Error(t, joined)
	got := NewDesc(joined).Stringify(false)
	want := `[1] error[E0004]: unknown error
Workbook: Items.xlsx
Worksheet: ItemConf
DataCellPos: C3
DataCell: abc
Reason: invalid integer

[2] error[E2000]: integer overflow
Workbook: Items.xlsx
Worksheet: ItemConf
DataCellPos: D3
DataCell: 999999999999
Reason: value "999999999999" is outside of range [-2147483648,2147483647] of type int32
Help: check field value and make sure it in representable range

[3] row5: duplicate key`
	assert.Equal(t, want, got)
}

func TestCollector_Stringify_MidLevelFull(t *testing.T) {
	global := NewCollector(100)
	book := global.NewChild(3)

	sheet1 := book.NewChild(10)
	_ = sheet1.Collect(fmt.Errorf("sheet1: err1"))
	_ = sheet1.Collect(fmt.Errorf("sheet1: err2"))
	assert.False(t, book.IsFull())

	sheet2 := book.NewChild(10)
	err := sheet2.Collect(fmt.Errorf("sheet2: err1"))
	require.Error(t, err)
	assert.True(t, book.IsFull())
	assert.True(t, sheet2.IsFull())
	assert.False(t, global.IsFull())

	joined := global.Join()
	require.Error(t, joined)
	got := NewDesc(joined).Stringify(false)
	want := `[1] sheet1: err1
[2] sheet1: err2
[3] sheet2: err1`
	assert.Equal(t, want, got)
}

func TestCollector_Stringify_NumberedList(t *testing.T) {
	global := NewCollector(10)
	book := global.NewChild(10)
	sheet := book.NewChild(10)

	_ = sheet.Collect(fmt.Errorf("error alpha"))
	_ = sheet.Collect(fmt.Errorf("error beta"))
	_ = sheet.Collect(fmt.Errorf("error gamma"))

	joined := global.Join()
	require.Error(t, joined)
	got := NewDesc(joined).Stringify(false)
	want := `[1] error alpha
[2] error beta
[3] error gamma`
	assert.Equal(t, want, got)
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
	_ = c.Collect(fmt.Errorf("err 1"))

	joined := c.Join()
	assert.Error(t, joined)

	var ce *collected
	assert.True(t, errors.As(joined, &ce), "Join() should return a collected-wrapped error")
}

// Collect treats a foreign collected error (from an unrelated collector
// tree) as an ordinary error so it is not silently dropped.
func TestCollected_CollectForeignCollectedIsNotDropped(t *testing.T) {
	c := NewCollector(10)
	_ = c.Collect(fmt.Errorf("err 1"))

	joined := c.Join()
	assert.Error(t, joined)

	// Create a new, unrelated collector and Collect the joined error.
	// Since joined.origin (= c) is not in c2's tree, c2 must keep it
	// rather than silently drop it.
	c2 := NewCollector(10)
	_ = c2.Collect(joined)
	got := c2.Join()
	assert.Error(t, got, "foreign collected error must not be silently dropped")
	assert.Contains(t, got.Error(), "err 1")
}

// Collect treats a WrapKV'd foreign collected error as an ordinary error
// so it is not silently dropped; the wrapping fields remain available.
func TestCollected_CollectForeignWrappedCollectedIsNotDropped(t *testing.T) {
	c := NewCollector(10)
	_ = c.Collect(fmt.Errorf("err 1"))

	joined := c.Join()
	wrapped := WrapKV(joined, KeyBookName, "test.xlsx")

	c2 := NewCollector(10)
	_ = c2.Collect(wrapped)
	got := c2.Join()
	assert.Error(t, got, "WrapKV'd foreign collected error must not be silently dropped")
	assert.Contains(t, got.Error(), "err 1")
}

// Collecting a WrapKV'd same-tree collected error records the outer wrapper's
// scope fields on the originating collector so Join() can re-apply them.
func TestCollected_CollectSameTreeWrappedCollectedRecordsOuterFields(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0)
	_ = child.Collect(fmt.Errorf("err 1"))

	joined := child.Join()
	assert.Error(t, joined)
	wrapped := WrapKV(joined, KeyBookName, "test.xlsx")

	// root and child share a tree, so the collected marker is recognized
	// and the outer WrapKV layer is recorded on child (origin) for Join().
	_ = root.Collect(wrapped)

	if assert.NotNil(t, child.outerFields, "outer scope fields should be recorded on the originating collector") {
		assert.Equal(t, "test.xlsx", child.outerFields[KeyBookName])
	}

	// child.Join() re-applies the recorded fields, surfacing them in its Desc.
	rejoined := child.Join()
	assert.NotNil(t, rejoined)
	assert.Equal(t, "test.xlsx", NewDesc(rejoined).fields[KeyBookName])
}

// Collecting a same-tree collected error when the receiver is already full
// returns the joined error tree immediately (early termination).
func TestCollected_CollectSameTreeCollectedWhenFullReturnsJoin(t *testing.T) {
	root := NewCollector(1) // fail-fast: full after the first error
	child := root.NewChild(0)
	_ = child.Collect(fmt.Errorf("err 1"))
	assert.True(t, root.IsFull(), "root should be full after child collects one error")

	joined := child.Join()
	assert.Error(t, joined)

	// Same-tree collected marker is recognized; root is full, so Collect
	// short-circuits and returns root.Join() instead of nil.
	got := root.Collect(joined)
	assert.Error(t, got, "full collector should return Join() for a same-tree collected error")
}

// sameTreeAs handles nil receivers/arguments defensively.
func TestCollector_SameTreeAs_Nil(t *testing.T) {
	c := NewCollector(10)
	assert.False(t, c.sameTreeAs(nil), "nil other should not be same tree")

	var nilCollector *Collector
	assert.False(t, nilCollector.sameTreeAs(c), "nil receiver should not be same tree")
	assert.False(t, nilCollector.sameTreeAs(nil), "nil receiver and nil other should not be same tree")
}

// collected marker is transparent: Error() delegates to inner.
func TestCollected_ErrorDelegates(t *testing.T) {
	c := NewCollector(10)
	_ = c.Collect(fmt.Errorf("hello"))

	joined := c.Join()
	assert.Contains(t, joined.Error(), "hello")
}

// collected marker is transparent to field propagation: outer WrapKV fields
// must reach the joined children when rendering via Error(), so the
// module-specific template (confgen) is used instead of the default one.
//
// This mirrors the real confgen chain:
//
//	parseFieldValue             -> E2002
//	tableParser.Parse           -> WrapKV(DataCellPos/DataCell/ColumnName)
//	sheetCollector.Join         -> collected{withMessage{joinError}}
//	sheetParser.Parse           -> WrapKV(Module)
//	parseMessageFromOneImporter -> WrapKV(Module/BookName/SheetName)
func TestCollected_ErrorPropagatesOuterFields(t *testing.T) {
	cellErr := WrapKV(E2002("100033333", "ItemConf.ID"),
		KeyDataCellPos, "F12",
		KeyDataCell, "100033333",
		KeyColumnName, "ItemID",
	)

	child := NewCollector(10).NewChild(5)
	_ = child.Collect(cellErr)

	err := WrapKV(WrapKV(child.Join(), KeyModule, ModuleConf),
		KeyModule, ModuleConf,
		KeyBookName, "Activity.xlsx",
		KeySheetName, "SectionConf",
	)

	want := `error[E2002]: field value not in referred space
Workbook: Activity.xlsx
Worksheet: SectionConf
DataCellPos: F12
DataCell: 100033333
Reason: value "100033333" not in referred space "ItemConf.ID"
Help: guarantee value "100033333" was configured in referred space "ItemConf.ID" ahead
`
	assert.Equal(t, want, err.Error())
}

// Re-collecting an already-collected same-tree error must preserve the fields
// of every wrapper layer, not just the outermost one.
//
// This mirrors the real confgen chain of a merger/scatter sheet, where Module
// and BookName/SheetName are added by different layers:
//
//	tableParser.Parse           -> sheetCollector.Join() == collected
//	sheetParser.Parse           -> WrapKV(Module)
//	parseMessageFromOneImporter -> WrapKV(Module/BookName/SheetName/PBMessage)
//	ParseMessage's Group.Go     -> WrapKV(BookName/SheetName/Primary*)  <- outermost, no Module
//	Group.Wait                  -> bookCollector.Join()
func TestCollected_ReCollectPreservesAllWrapperFields(t *testing.T) {
	cellErr := WrapKV(E2002("100033333", "ItemConf.ID"),
		KeyDataCellPos, "F12",
		KeyDataCell, "100033333",
	)

	book := NewCollector(10)
	sheet := book.NewChild(5)
	_ = sheet.Collect(cellErr)

	// Module is added by an intermediate layer, while the outermost layer only
	// carries book/sheet names.
	err := WrapKV(WrapKV(sheet.Join(),
		KeyModule, ModuleConf,
		KeyBookName, "Activity.xlsx",
		KeySheetName, "SectionConf",
	), KeyPrimaryBookName, "Activity.xlsx", KeyPrimarySheetName, "SectionConf")
	_ = book.Collect(err)

	want := `error[E2002]: field value not in referred space
Workbook: Activity.xlsx
Worksheet: SectionConf
DataCellPos: F12
DataCell: 100033333
Reason: value "100033333" not in referred space "ItemConf.ID"
Help: guarantee value "100033333" was configured in referred space "ItemConf.ID" ahead
`
	assert.Equal(t, want, book.Join().Error())
}

// Only scope fields (which book/sheet/message) may be broadcast to the joined
// children; per-cell fields belong to a single error. Recording a wrapper's
// DataCellPos/DataCell would point every sibling at a cell it never touched.
//
// This mirrors the real confgen chain of a horizontal map, where the wrapper
// around the nested join carries the *key* column's cell:
//
//	parseMessage               -> messageCollector.Join() == collected
//	parseHorizontalMapField    -> WrapKV(CellDebugKV of the key column)
//	parseMessage (parent)      -> messageCollector.Collect(...)  <- same tree
func TestCollected_ReCollectDropsNonScopeFields(t *testing.T) {
	sheet := NewCollector(10)
	nested := sheet.NewChild(5)
	// Sibling 1 owns its cell; sibling 2 has no cell of its own.
	_ = nested.Collect(WrapKV(E2002("100033333", "ItemConf.ID"),
		KeyDataCellPos, "B4",
		KeyDataCell, "100033333",
	))
	_ = nested.Collect(E2014("Item1Miss"))

	// The wrapper carries the key column's cell alongside the scope fields.
	_ = sheet.Collect(WrapKV(nested.Join(),
		KeyModule, ModuleConf,
		KeyBookName, "Activity.xlsx",
		KeySheetName, "SectionConf",
		KeyDataCellPos, "A4",
		KeyDataCell, "7",
		KeyColumnName, "Item1ID",
	))

	got := sheet.Join().Error()
	// Scope fields are broadcast to both children.
	assert.Contains(t, got, "Workbook: Activity.xlsx")
	assert.Contains(t, got, "Worksheet: SectionConf")
	// Sibling 1 keeps its own cell; the wrapper's cell reaches neither.
	assert.Contains(t, got, "DataCellPos: B4")
	assert.NotContains(t, got, "DataCellPos: A4",
		"wrapper cell position must not be broadcast to the joined children")
	assert.NotContains(t, got, "DataCell: 7",
		"wrapper cell data must not be broadcast to the joined children")
}

// A wrapper carrying no scope fields must clear the previously recorded ones,
// so a stale book/sheet name is never re-applied to an unrelated join.
func TestCollected_ReCollectFieldlessWrapperClearsStaleFields(t *testing.T) {
	book := NewCollector(10)
	first := book.NewChild(5)
	_ = first.Collect(E2002("v1", "ItemConf.ID"))
	_ = book.Collect(WrapKV(book.Join(),
		KeyModule, ModuleConf,
		KeyBookName, "First.xlsx",
		KeySheetName, "S1",
	))

	second := book.NewChild(5)
	_ = second.Collect(E2002("v2", "ShopConf.ID"))
	_ = book.Collect(Wrap(book.Join()))

	got := book.Join().Error()
	assert.NotContains(t, got, "First.xlsx", "stale book name must not survive")
	assert.NotContains(t, got, "Worksheet: S1", "stale sheet name must not survive")
}

// collected marker is transparent: errors.Is works through it.
func TestCollected_ErrorsIsWorksThrough(t *testing.T) {
	target := fmt.Errorf("target")
	c := NewCollector(10)
	_ = c.Collect(target)

	joined := c.Join()
	assert.True(t, errors.Is(joined, target), "errors.Is should see through collected marker")
}

// Simulates recursive parseMessage: inner fieldChild.Join() flows to outer
// fieldChild.Collect(), which skips it. Parent's Join() includes both via
// tree auto-join.
//
//	root
//	└── docChild
//	    ├── outerChild  (skips innerJoined)
//	    └── innerChild  (has "inner err 1", "inner err 2")
func TestCollected_CrossSubtreeCollection(t *testing.T) {
	root := NewCollector(10)
	docChild := root.NewChild(0)
	outerChild := docChild.NewChild(0)
	innerChild := docChild.NewChild(0)

	_ = innerChild.Collect(fmt.Errorf("inner err 1"))
	_ = innerChild.Collect(fmt.Errorf("inner err 2"))

	innerJoined := innerChild.Join()
	assert.Error(t, innerJoined)

	// Outer Collect skips it (collected marker).
	_ = outerChild.Collect(innerJoined)

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

	_ = child.Collect(fmt.Errorf("err 1"))
	_ = child.Collect(fmt.Errorf("err 2"))

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
	_ = c.Collect(fmt.Errorf("err"))
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
	_ = child.Collect(fmt.Errorf("child err"))

	assert.True(t, root.HasErrors())
}

// HasErrors detects errors in grandchildren.
func TestHasErrors_ErrorInGrandchild(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0)
	gc := child.NewChild(0)
	_ = gc.Collect(fmt.Errorf("deep err"))

	assert.True(t, root.HasErrors())
	assert.True(t, child.HasErrors())
	assert.True(t, gc.HasErrors())
}
