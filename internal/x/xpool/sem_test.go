package xpool

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSemaphore_Cap(t *testing.T) {
	s := NewSemaphore(4)
	if got, want := s.Cap(), 4; got != want {
		t.Fatalf("Cap() = %d, want %d", got, want)
	}
	if got := s.Len(); got != 0 {
		t.Fatalf("Len() of fresh sem = %d, want 0", got)
	}
}

func TestSemaphore_NewClampsBelowOne(t *testing.T) {
	for _, n := range []int{0, -1, -100} {
		s := NewSemaphore(n)
		if s.Cap() != 1 {
			t.Fatalf("NewSemaphore(%d).Cap() = %d, want 1 (clamped)", n, s.Cap())
		}
	}
}

func TestSemaphore_AcquireRelease(t *testing.T) {
	s := NewSemaphore(2)
	ctx := context.Background()
	if err := s.Acquire(ctx); err != nil {
		t.Fatalf("Acquire 1: %v", err)
	}
	if err := s.Acquire(ctx); err != nil {
		t.Fatalf("Acquire 2: %v", err)
	}
	if got := s.Len(); got != 2 {
		t.Fatalf("Len() = %d, want 2", got)
	}
	s.Release()
	s.Release()
	if got := s.Len(); got != 0 {
		t.Fatalf("Len() after release = %d, want 0", got)
	}
}

func TestSemaphore_TryAcquire(t *testing.T) {
	s := NewSemaphore(1)
	if !s.TryAcquire() {
		t.Fatal("first TryAcquire failed")
	}
	if s.TryAcquire() {
		t.Fatal("second TryAcquire on full sem must fail")
	}
	s.Release()
	if !s.TryAcquire() {
		t.Fatal("TryAcquire after release should succeed")
	}
	s.Release()
}

func TestSemaphore_AcquireBlocksThenWakesOnRelease(t *testing.T) {
	s := NewSemaphore(1)
	if err := s.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_ = s.Acquire(context.Background())
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("second Acquire returned before slot was released")
	case <-time.After(20 * time.Millisecond):
	}
	s.Release()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("second Acquire did not wake up after Release")
	}
	s.Release()
}

func TestSemaphore_AcquireCancellation(t *testing.T) {
	s := NewSemaphore(1)
	if err := s.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- s.Acquire(ctx) }()
	cancel()
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("Acquire after cancel returned nil error")
		}
	case <-time.After(time.Second):
		t.Fatal("Acquire did not unblock after cancel")
	}
	// Verify the sem is still usable and not corrupted: capacity unchanged.
	if got := s.Len(); got != 1 {
		t.Fatalf("Len() = %d, want 1 (cancelled acquire must not consume a slot)", got)
	}
	s.Release()
}

// TestSemaphore_BoundsConcurrency stresses the semaphore with many
// goroutines and asserts the in-flight count never exceeds capacity.
func TestSemaphore_BoundsConcurrency(t *testing.T) {
	const cap = 4
	const workers = 64
	s := NewSemaphore(cap)
	var inFlight int32
	var maxInFlight int32
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			if err := s.Acquire(context.Background()); err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			defer s.Release()
			cur := atomic.AddInt32(&inFlight, 1)
			for {
				old := atomic.LoadInt32(&maxInFlight)
				if cur <= old || atomic.CompareAndSwapInt32(&maxInFlight, old, cur) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&maxInFlight); got > cap {
		t.Fatalf("max concurrent holders = %d, exceeds cap = %d", got, cap)
	}
}

func TestSemaphore_Run(t *testing.T) {
	s := NewSemaphore(1)
	called := false
	err := s.Run(context.Background(), func() error {
		called = true
		if got := s.Len(); got != 1 {
			t.Errorf("Len inside fn = %d, want 1", got)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !called {
		t.Fatal("Run did not invoke fn")
	}
	if got := s.Len(); got != 0 {
		t.Fatalf("Len after Run = %d, want 0", got)
	}
}

func TestSemaphore_RunCancelledBeforeAcquire(t *testing.T) {
	s := NewSemaphore(1)
	if err := s.Acquire(context.Background()); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer s.Release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	err := s.Run(ctx, func() error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("Run with cancelled ctx returned nil error")
	}
	if called {
		t.Fatal("Run invoked fn even though Acquire failed")
	}
}

func TestNewCPUSemaphore(t *testing.T) {
	s := NewCPUSemaphore()
	if s.Cap() < 1 {
		t.Fatalf("NewCPUSemaphore Cap = %d, want >= 1", s.Cap())
	}
}
