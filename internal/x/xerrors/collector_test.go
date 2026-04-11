package xerrors

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollector_NilError(t *testing.T) {
	c := NewCollector(10)
	full, err := c.Collect(nil)
	assert.False(t, full)
	assert.NoError(t, err)
	assert.NoError(t, c.Join())
}

func TestCollector_SingleError(t *testing.T) {
	c := NewCollector(10)
	want := errors.New("single error")
	full, err := c.Collect(want)
	assert.False(t, full)
	assert.NoError(t, err)
	assert.ErrorIs(t, c.Join(), want)
}

func TestCollector_MaxErrors(t *testing.T) {
	const max = 3
	c := NewCollector(max)

	for i := range 5 {
		full, joinedErr := c.Collect(fmt.Errorf("error %d", i))
		if i < max-1 {
			// under limit: accepted, not yet full
			assert.False(t, full, "Collect error %d: full should be false (under limit)", i)
			assert.NoError(t, joinedErr, "Collect error %d: err should be nil (under limit)", i)
		} else {
			// at or over limit: full, joined errors returned
			assert.True(t, full, "Collect error %d: full should be true (at/over limit)", i)
			assert.Error(t, joinedErr, "Collect error %d: err should be non-nil (at/over limit)", i)
		}
	}

	joined := c.Join()
	assert.NotNil(t, joined)
	// Verify exactly max errors are present.
	assert.Equal(t, max, countJoinedErrors(joined))
}

func TestCollector_ConcurrentCollect(t *testing.T) {
	const max = 10
	const total = 50
	c := NewCollector(max)

	var wg sync.WaitGroup
	for i := range total {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Collect(fmt.Errorf("error %d", i))
		}()
	}
	wg.Wait()

	joined := c.Join()
	assert.NotNil(t, joined, "Join() should not be nil after concurrent collects")
	assert.Equal(t, max, countJoinedErrors(joined), "expected exactly %d errors (max)", max)
}

func TestCollector_JoinIsNilWhenEmpty(t *testing.T) {
	c := NewCollector(5)
	assert.NoError(t, c.Join())
}

func TestCollector_IsFull(t *testing.T) {
	const max = 3
	c := NewCollector(max)

	assert.False(t, c.IsFull(), "IsFull() should be false on a fresh collector")

	for i := range max - 1 {
		full, _ := c.Collect(fmt.Errorf("error %d", i))
		assert.False(t, full, "Collect error %d: full should be false (%d/%d)", i, i+1, max)
		assert.False(t, c.IsFull(), "IsFull() should be false after %d/%d errors", i+1, max)
	}

	// Collect the last allowed error — now full.
	full, joinedErr := c.Collect(fmt.Errorf("error %d", max-1))
	assert.True(t, full, "Collect at max: full should be true")
	assert.Error(t, joinedErr, "Collect at max: err should be non-nil")
	assert.True(t, c.IsFull(), "IsFull() should be true after reaching max errors")

	// Collecting beyond max keeps it full.
	full, joinedErr = c.Collect(fmt.Errorf("overflow error"))
	assert.True(t, full, "Collect beyond max: full should be true")
	assert.Error(t, joinedErr, "Collect beyond max: err should be non-nil")
	assert.True(t, c.IsFull(), "IsFull() should remain true after exceeding max errors")
}

// TestCollector_ZeroMax_Unlimited verifies that maxErrs <= 0 means unlimited.
func TestCollector_ZeroMax_Unlimited(t *testing.T) {
	c := NewCollector(0)
	for i := range 5 {
		full, err := c.Collect(fmt.Errorf("error %d", i))
		assert.False(t, full, "Collect error %d: full should be false when unlimited", i)
		assert.NoError(t, err, "Collect error %d: err should be nil when unlimited", i)
	}
	assert.False(t, c.IsFull(), "IsFull() should never be true when unlimited")
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
