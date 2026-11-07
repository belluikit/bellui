//go:build !wasm

package glfw

import (
	"sync"
	"testing"
	"time"
)

// TestCloseFuncQueue_WhilePosting pins #1021: work posted from other
// goroutines while shutdown closes the main queue. A post that had seen the
// queue open sent on it after the close and panicked ("send on closed
// channel"), and a waiting post whose work had not reached Out() yet was never
// released.
//
// Each round closes a queue of its own. TestMain's driver runs a live loop
// reading funcQueue, and closing that one handed the loop a zero funcData,
// whose nil f it called (#1063).
func TestCloseFuncQueue_WhilePosting(t *testing.T) {
	for range 200 {
		q := newMainQueue()

		// Each poster stops once it sees the queue drained: the posts that
		// race the close are the ones before that. Spinning on past it, eight
		// posters running their work in place starved the drain of the queue's
		// backlog on a CI runner under -race, for ten minutes.
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Go(func() {
				for !q.drained.Load() {
					q.post(func() {}, i%2 == 0)
				}
			})
		}
		time.Sleep(200 * time.Microsecond)
		q.close()

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
		if n := q.pending.Load(); n != 0 {
			t.Fatalf("%d posts still counted pending after the close", n)
		}
	}
}
