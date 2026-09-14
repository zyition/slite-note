//go:build windows

package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// short delay so tests stay fast; behaviour is delay-independent.
const testTrimDelay = 40 * time.Millisecond

// newTestDebounce returns a debouncer whose trim is a counting stub and whose
// visibility precondition can be flipped by the test.
func newTestDebounce(delay time.Duration) (*trimDebounce, *int32, *atomic.Bool) {
	var runs int32
	hidden := &atomic.Bool{}
	hidden.Store(true)
	d := &trimDebounce{
		delay:        delay,
		windowHidden: func() bool { return hidden.Load() },
		run:          func() { atomic.AddInt32(&runs, 1) },
	}
	return d, &runs, hidden
}

// waitUntil polls cond until it holds or the timeout elapses.
func waitUntil(t *testing.T, cond func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v", timeout)
}

// Core debounce contract: a burst of overlapping requests coalesces into
// exactly one run, delay after the LAST request (not the first).
func TestTrimDebounceBurstRunsOnce(t *testing.T) {
	d, runs, _ := newTestDebounce(testTrimDelay)

	var wg sync.WaitGroup
	// Half the callers sequential, half concurrent: the sequential half
	// exercises Reset-while-armed, the concurrent half the mutex itself.
	for i := 0; i < 10; i++ {
		d.schedule()
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.schedule()
		}()
	}
	wg.Wait()

	waitUntil(t, func() bool { return atomic.LoadInt32(runs) == 1 }, 2*time.Second)
	// No second run may leak out after the burst.
	time.Sleep(3 * testTrimDelay)
	if got := atomic.LoadInt32(runs); got != 1 {
		t.Fatalf("burst of 21 schedule() calls → %d runs, want exactly 1", got)
	}
}

// A later request must re-arm the timer: nothing may fire delay after the
// FIRST request if a second request arrived in between. Verified by checking
// the run lands at least delay after the second call (a non-reset timer would
// have fired ~delay after the first).
func TestTrimDebounceLaterCallRearms(t *testing.T) {
	d, runs, _ := newTestDebounce(testTrimDelay)

	d.schedule()
	time.Sleep(testTrimDelay / 2)
	second := time.Now()
	d.schedule()

	waitUntil(t, func() bool { return atomic.LoadInt32(runs) == 1 }, 2*time.Second)
	if got := atomic.LoadInt32(runs); got != 1 {
		t.Fatalf("got %d runs, want 1", got)
	}
	firedAfterReset := time.Since(second)
	if firedAfterReset < testTrimDelay*8/10 {
		t.Fatalf("fired %v after the second call; timer was not re-armed (want ≥ %v)",
			firedAfterReset, testTrimDelay*8/10)
	}
}

// The trim is gated on the window being hidden at fire time: a re-show within
// the delay window cancels the pending trim entirely.
func TestTrimDebounceSkipsWhenWindowVisible(t *testing.T) {
	d, runs, hidden := newTestDebounce(testTrimDelay)

	d.schedule()
	hidden.Store(false) // user summons the window before the timer fires
	time.Sleep(3 * testTrimDelay)

	if got := atomic.LoadInt32(runs); got != 0 {
		t.Fatalf("trim ran with the window visible; want skipped (runs=%d)", got)
	}

	// Hiding again afterwards must still work (timer was consumed).
	hidden.Store(true)
	d.schedule()
	waitUntil(t, func() bool { return atomic.LoadInt32(runs) == 1 }, 2*time.Second)
}

// The timer must be re-usable after every run: one trim per hide for the
// whole app lifetime, not just the first hide.
func TestTrimDebounceReusableAcrossRuns(t *testing.T) {
	d, runs, _ := newTestDebounce(testTrimDelay)

	for round := 1; round <= 3; round++ {
		d.schedule()
		want := int32(round)
		waitUntil(t, func() bool { return atomic.LoadInt32(runs) == want }, 2*time.Second)
	}
	if got := atomic.LoadInt32(runs); got != 3 {
		t.Fatalf("got %d runs, want 3 (one per hide/trim cycle)", got)
	}
}

// Stress the reset/callback race: concurrent schedulers interleaved with the
// timer firing. Run with -race; the assertion is just that runs stay sane
// (each round eventually fires exactly once).
func TestTrimDebounceConcurrentStress(t *testing.T) {
	d, runs, _ := newTestDebounce(testTrimDelay)

	for round := 1; round <= 5; round++ {
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				d.schedule()
				time.Sleep(time.Millisecond)
				d.schedule()
			}()
		}
		wg.Wait()
		want := int32(round)
		waitUntil(t, func() bool { return atomic.LoadInt32(runs) == want }, 2*time.Second)
	}
}
