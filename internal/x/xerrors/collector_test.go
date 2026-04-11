package xerrors

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestCollector_NilError(t *testing.T) {
	c := NewCollector(context.Background(), 10)
	if c.Add(nil) {
		t.Error("Add(nil) should return false")
	}
	if err := c.Join(); err != nil {
		t.Errorf("Join() should be nil for no errors, got: %v", err)
	}
}

func TestCollector_SingleError(t *testing.T) {
	c := NewCollector(context.Background(), 10)
	want := errors.New("single error")
	if !c.Add(want) {
		t.Error("Add should return true for the first error")
	}
	if got := c.Join(); got == nil {
		t.Error("Join() should not be nil")
	} else if !errors.Is(got, want) {
		t.Errorf("Join() = %v, want %v", got, want)
	}
}

func TestCollector_MaxErrors(t *testing.T) {
	const max = 3
	c := NewCollector(context.Background(), max)

	for i := range 5 {
		err := fmt.Errorf("error %d", i)
		accepted := c.Add(err)
		if i < max && !accepted {
			t.Errorf("Add error %d should be accepted (under limit)", i)
		}
		if i >= max && accepted {
			t.Errorf("Add error %d should be rejected (over limit)", i)
		}
	}

	joined := c.Join()
	if joined == nil {
		t.Fatal("Join() should not be nil")
	}
	// Verify exactly max errors are present.
	count := countJoinedErrors(joined)
	if count != max {
		t.Errorf("expected %d joined errors, got %d", max, count)
	}
}

func TestCollector_ConcurrentAdd(t *testing.T) {
	const max = 10
	const total = 50
	c := NewCollector(context.Background(), max)

	var wg sync.WaitGroup
	for i := range total {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Add(fmt.Errorf("error %d", i))
		}()
	}
	wg.Wait()

	joined := c.Join()
	if joined == nil {
		t.Fatal("Join() should not be nil after concurrent adds")
	}
	count := countJoinedErrors(joined)
	if count != max {
		t.Errorf("expected exactly %d errors (max), got %d", max, count)
	}
}

func TestCollector_ZeroMax(t *testing.T) {
	c := NewCollector(context.Background(), 0)
	if c.Add(errors.New("any")) {
		t.Error("Add should return false when maxErrs is 0")
	}
	if err := c.Join(); err != nil {
		t.Errorf("Join() should be nil when maxErrs is 0, got: %v", err)
	}
}

func TestCollector_JoinIsNilWhenEmpty(t *testing.T) {
	c := NewCollector(context.Background(), 5)
	if err := c.Join(); err != nil {
		t.Errorf("Join() on empty collector should be nil, got: %v", err)
	}
}

func TestCollector_IsFull(t *testing.T) {
	const max = 3
	c := NewCollector(context.Background(), max)

	if c.IsFull() {
		t.Error("IsFull() should be false on a fresh collector")
	}

	for i := range max - 1 {
		c.Add(fmt.Errorf("error %d", i))
		if c.IsFull() {
			t.Errorf("IsFull() should be false after %d/%d errors", i+1, max)
		}
	}

	// Add the last allowed error — now full.
	c.Add(fmt.Errorf("error %d", max-1))
	if !c.IsFull() {
		t.Error("IsFull() should be true after reaching max errors")
	}

	// Adding beyond max keeps it full.
	c.Add(fmt.Errorf("overflow error"))
	if !c.IsFull() {
		t.Error("IsFull() should remain true after exceeding max errors")
	}
}

// TestCollector_Done verifies that Done() is closed as soon as the error limit
// is reached, allowing running goroutines to detect the condition immediately.
func TestCollector_Done(t *testing.T) {
	const max = 2
	c := NewCollector(context.Background(), max)

	// Done() must not be closed before the limit is reached.
	select {
	case <-c.Done():
		t.Fatal("Done() should not be closed before reaching max errors")
	default:
	}

	c.Add(fmt.Errorf("error 1"))
	select {
	case <-c.Done():
		t.Fatal("Done() should not be closed after only 1 of 2 errors")
	default:
	}

	// Adding the last allowed error must close Done().
	c.Add(fmt.Errorf("error 2"))
	select {
	case <-c.Done():
		// expected
	default:
		t.Fatal("Done() should be closed after reaching max errors")
	}
}

// TestCollector_DoneOnParentCancel verifies that Done() is also closed when
// the parent context is cancelled, even before the error limit is reached.
func TestCollector_DoneOnParentCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := NewCollector(ctx, 10)

	select {
	case <-c.Done():
		t.Fatal("Done() should not be closed before parent cancel")
	default:
	}

	cancel()

	select {
	case <-c.Done():
		// expected
	default:
		t.Fatal("Done() should be closed after parent context is cancelled")
	}
}

// TestCollector_GoroutineStopsOnDone verifies that goroutines watching Done()
// exit immediately once the error limit is reached by a sibling goroutine.
func TestCollector_GoroutineStopsOnDone(t *testing.T) {
	const max = 1
	c := NewCollector(context.Background(), max)

	var stopped sync.WaitGroup
	stopped.Add(1)
	go func() {
		defer stopped.Done()
		// Simulate a goroutine that checks Done() before doing work.
		select {
		case <-c.Done():
			return // fail-fast exit
		default:
		}
		// Should not reach here after Done() is closed.
		t.Error("goroutine should have exited via Done()")
	}()

	// Fill the collector to trigger cancel.
	c.Add(fmt.Errorf("trigger error"))

	// Now launch a goroutine that should exit immediately via Done().
	var wg sync.WaitGroup
	wg.Add(1)
	exited := false
	go func() {
		defer wg.Done()
		select {
		case <-c.Done():
			exited = true
			return
		default:
		}
	}()
	wg.Wait()
	stopped.Wait()

	if !exited {
		t.Error("goroutine should have exited via Done() after error limit reached")
	}
}

// countJoinedErrors counts the number of individual errors wrapped by errors.Join.
func countJoinedErrors(err error) int {
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
