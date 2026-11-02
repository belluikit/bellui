//go:build !wasm

package mobile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/internal/async"
)

// TestDoBeforeLoopStarts_OffMainQueues: a goroutine that reaches main before the
// run loop starts must wait for the loop, in order. Run on that goroutine, its
// widget changes race main's build of the UI (#890).
func TestDoBeforeLoopStarts_OffMainQueues(t *testing.T) {
	early := &driver{queuedFuncs: async.NewUnboundedChan[func()]()}
	var ran []string

	returned := make(chan struct{})
	go func() {
		early.DoFromGoroutine(func() { ran = append(ran, "do") }, false)
		early.DoFromGoroutine(func() { ran = append(ran, "wait") }, true)
		close(returned)
	}()

	next := func() func() {
		select {
		case fn := <-early.queuedFuncs.Out():
			return fn
		case <-time.After(5 * time.Second):
			require.Fail(t, "the call was not queued")
			return nil
		}
	}
	first, second := next(), next()
	select {
	case <-returned:
		t.Fatal("DoAndWait returned before its function ran")
	default:
	}
	assert.Empty(t, ran, "nothing runs until the loop does")

	first()
	second()
	<-returned
	assert.Equal(t, []string{"do", "wait"}, ran)
}

// TestDoBeforeLoopStarts_OnMainRunsInline: main has no loop to wait for yet, so
// its own call runs now. Queued, it would wait for main itself.
func TestDoBeforeLoopStarts_OnMainRunsInline(t *testing.T) {
	early := &driver{queuedFuncs: async.NewUnboundedChan[func()]()}
	var inline bool
	d.DoFromGoroutine(func() {
		early.DoFromGoroutine(func() { inline = true }, false)
	}, true)
	assert.True(t, inline)
}
