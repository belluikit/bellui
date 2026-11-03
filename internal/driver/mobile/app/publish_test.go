package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func setPublishWait(t *testing.T, d time.Duration) {
	t.Helper()
	old := publishWait
	publishWait = d
	t.Cleanup(func() { publishWait = old })
}

// A frame nobody takes must not park the caller once publishWait is set: on
// iOS the display link that takes it stops while the screen is locked, and
// the caller is the loop that runs bellui.Do work (#910).
func TestHandshake_UntakenFrameReturns(t *testing.T) {
	setPublishWait(t, 20*time.Millisecond)
	a := &app{publish: make(chan struct{}), publishResult: make(chan PublishResult)}

	done := make(chan struct{})
	go func() {
		a.handshake()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handshake still waiting for a frame nobody takes")
	}

	// The dropped frame must not be left for a later taker: a stale send
	// would hand the next display-link tick a frame and block it on a
	// result nobody reads.
	select {
	case <-a.publish:
		t.Fatal("a dropped frame was still offered")
	default:
	}
}

// A frame that is taken waits for the taker's result, however long publishWait.
func TestHandshake_TakenFrameGetsResult(t *testing.T) {
	setPublishWait(t, time.Hour)
	a := &app{publish: make(chan struct{}), publishResult: make(chan PublishResult)}

	go func() {
		<-a.publish
		a.publishResult <- PublishResult{BackBufferPreserved: true}
	}()
	assert.True(t, a.handshake().BackBufferPreserved)
}

// Without publishWait the handshake waits as long as it takes: where taking a
// frame presents it, dropping one would leave the screen stale.
func TestHandshake_UnboundedWaitsForTaker(t *testing.T) {
	setPublishWait(t, 0)
	a := &app{publish: make(chan struct{}), publishResult: make(chan PublishResult)}

	done := make(chan PublishResult)
	go func() { done <- a.handshake() }()
	select {
	case <-done:
		t.Fatal("unbounded handshake returned with no taker")
	case <-time.After(100 * time.Millisecond):
	}
	<-a.publish
	a.publishResult <- PublishResult{BackBufferPreserved: true}
	assert.True(t, (<-done).BackBufferPreserved)
}
