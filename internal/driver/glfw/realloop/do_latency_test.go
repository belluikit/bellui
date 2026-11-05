//go:build darwin && !ci

package realloop_test

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/belluikit/bellui"
)

// TestDoAndWaitDoesNotSleepThroughWork pins #981 (found as player#287): Do put work on the
// main queue's In side and woke the loop at once, but a separate goroutine
// moves it to the Out side the loop drains. When that goroutine ran late, the
// loop woke, found nothing, and slept a whole idle tick (a second) with the
// work queued, so a DoAndWait could stall for up to a second on a busy
// machine.
//
// One busy goroutine per P makes the forwarding goroutine late. It also
// delays the main goroutine getting a P back, which is what the fixed loop
// still shows. Measured 2026-11-05, worst call of 400 a run: before the fix
// 647-987 ms (5 runs); after it 93-381 ms (15 runs). The limit sits between.
func TestDoAndWaitDoesNotSleepThroughWork(t *testing.T) {
	var stop atomic.Bool
	defer stop.Store(true)
	for range runtime.NumCPU() {
		go func() {
			x := 0
			for !stop.Load() {
				x++
			}
		}()
	}

	const calls = 400
	var worst time.Duration
	for range calls {
		// A gap long enough for the loop to go idle and wait again.
		time.Sleep(3 * time.Millisecond)
		start := time.Now()
		bellui.DoAndWait(func() {})
		worst = max(worst, time.Since(start))
	}
	if worst > 550*time.Millisecond {
		t.Fatalf("a DoAndWait under load took %v (worst of %d): the main loop slept with work queued", worst, calls)
	}
	t.Logf("worst DoAndWait of %d under load: %v", calls, worst)
}
