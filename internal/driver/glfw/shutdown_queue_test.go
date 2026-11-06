//go:build !wasm

package glfw

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/belluikit/bellui/internal/async"
)

// TestCloseFuncQueue_WhilePosting pins #1021: work posted from other
// goroutines while shutdown closes the main queue. A post that had seen the
// queue open sent on it after the close and panicked ("send on closed
// channel"), and a waiting post whose work had not reached Out() yet was never
// released.
func TestCloseFuncQueue_WhilePosting(t *testing.T) {
	q, wasRunning, wasDrained := funcQueue, running.Load(), drained.Load()
	t.Cleanup(func() {
		funcQueue = q
		running.Store(wasRunning)
		drained.Store(wasDrained)
	})

	for range 200 {
		funcQueue = async.NewUnboundedChan[funcData]()
		running.Store(true)
		drained.Store(false)
		pendingFuncs.Store(0)

		var stop atomic.Bool
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Go(func() {
				for !stop.Load() {
					runOnMainWithWait(func() {}, i%2 == 0)
				}
			})
		}
		time.Sleep(200 * time.Microsecond)
		closeFuncQueue()
		stop.Store(true)

		returned := make(chan struct{})
		go func() {
			wg.Wait()
			close(returned)
		}()
		select {
		case <-returned:
		case <-time.After(5 * time.Second):
			t.Fatal("a post made while the queue closed never returned")
		}
	}
}
