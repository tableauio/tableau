// Package xpool provides lightweight concurrency-limiting primitives shared
// across confgen / protogen pipelines.
//
// The package intentionally exposes a small surface:
//
//   - [Semaphore] is a counting semaphore implemented as a buffered channel.
//     It is the building block for "at most N goroutines may execute the
//     critical section concurrently" patterns. Acquisitions are
//     context-cancellable so that errgroup-style fan-out propagates
//     cancellation correctly.
//
// Why a custom type instead of golang.org/x/sync/semaphore? The standard
// package supports weighted acquisitions and uses a sync.Mutex + waiter
// list internally; we only ever acquire weight=1, and the chan-based
// implementation is both simpler and faster on the hot path (single
// channel send / receive, zero allocations after construction).
package xpool

import (
	"context"
	"runtime"
)

// Semaphore is a counting semaphore that bounds the number of goroutines
// allowed to hold a "slot" simultaneously. It is safe for concurrent use
// by multiple goroutines.
//
// The zero value is NOT usable; construct via [NewSemaphore] or
// [NewCPUSemaphore].
type Semaphore struct {
	slots chan struct{}
}

// NewSemaphore constructs a semaphore with the given capacity. capacity
// must be >= 1; values < 1 are clamped to 1 so that the semaphore always
// admits at least one holder (the alternative -- a deadlocked semaphore --
// would silently break callers that compute capacity from environment
// signals like GOMAXPROCS in degenerate setups).
func NewSemaphore(capacity int) *Semaphore {
	if capacity < 1 {
		capacity = 1
	}
	return &Semaphore{slots: make(chan struct{}, capacity)}
}

// NewCPUSemaphore constructs a semaphore sized to runtime.GOMAXPROCS(0),
// the canonical "number of cores Go is allowed to use" signal that is
// cgroup-aware on recent Go releases. This is the right size for CPU-
// bound critical sections.
func NewCPUSemaphore() *Semaphore {
	return NewSemaphore(runtime.GOMAXPROCS(0))
}

// Cap reports the configured capacity.
func (s *Semaphore) Cap() int { return cap(s.slots) }

// Len reports the number of slots currently held. Intended for debugging
// / observability; the value is approximate under concurrent mutation.
func (s *Semaphore) Len() int { return len(s.slots) }

// Acquire blocks until a slot is available or ctx is cancelled. On
// success it returns nil and the caller MUST call [Semaphore.Release]
// exactly once when done. On cancellation it returns ctx.Err() and the
// caller MUST NOT call Release.
func (s *Semaphore) Acquire(ctx context.Context) error {
	select {
	case s.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryAcquire attempts to take a slot without blocking. It returns true
// on success (in which case [Semaphore.Release] must be called) or
// false if no slot was immediately available.
func (s *Semaphore) TryAcquire() bool {
	select {
	case s.slots <- struct{}{}:
		return true
	default:
		return false
	}
}

// Release returns a previously acquired slot. It is the caller's
// responsibility to ensure Release is called at most once per successful
// Acquire / TryAcquire; a stray Release will silently consume a slot
// from another holder, manifesting as a hard-to-diagnose hang.
func (s *Semaphore) Release() { <-s.slots }

// Run is a convenience helper that acquires a slot, runs fn, then
// releases the slot. If ctx is cancelled before a slot becomes available
// fn is NOT invoked and ctx.Err() is returned.
func (s *Semaphore) Run(ctx context.Context, fn func() error) error {
	if err := s.Acquire(ctx); err != nil {
		return err
	}
	defer s.Release()
	return fn()
}
