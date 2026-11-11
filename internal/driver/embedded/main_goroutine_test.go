package embedded_test

import (
	"image"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/embedded"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/build"
	intNoos "github.com/belluikit/bellui/internal/driver/embedded"
	"github.com/belluikit/bellui/widget"
)

// The driver's loop is the main goroutine, as the glfw and mobile drivers
// record theirs (#1132). Unrecorded, a wait queued from the loop itself
// deadlocked it, and under -tags mobile, where an unrecorded main counts every
// goroutine as main, a wait from any goroutine returned before its function
// ran.
func TestNoOSDriver_LoopIsMainGoroutine(t *testing.T) {
	events := make(chan embedded.Event)
	size := func() geo.Size { return geo.NewSquareSize(100) }
	d := intNoos.NewNoOSDriver(func(image.Image) {}, func(f func()) { f() }, events, size)
	d.CreateWindow("main").SetContent(widget.NewLabel(""))
	stopped := make(chan struct{})
	go func() {
		d.Run()
		close(stopped)
	}()
	t.Cleanup(func() {
		close(events)
		select {
		case <-stopped:
		case <-time.After(5 * time.Second):
			t.Error("the driver loop did not stop: it is stuck")
		}
	})
	// The queue is unbuffered: once this send returns, the loop is running.
	d.DoFromGoroutine(func() {}, false)

	assert.False(t, bellui.IsMainGoroutine(), "the test's goroutine is not main")
	var onLoop bool
	d.DoFromGoroutine(func() { onLoop = bellui.IsMainGoroutine() }, true)
	assert.True(t, onLoop, "the loop is main, and the wait ran the function before returning")

	if build.DisableThreadChecks {
		return // migrated_belluido turns the guard below off: the caller promises not to wait on main
	}
	// A wait from the loop itself is logged and run off the loop, not
	// deadlocked on its own queue.
	ran := make(chan struct{})
	returned := make(chan struct{})
	d.DoFromGoroutine(func() {
		d.DoFromGoroutine(func() { close(ran) }, true)
		close(returned)
	}, false)
	for name, ch := range map[string]chan struct{}{"the wait from the loop returned": returned, "its function ran": ran} {
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatalf("%s: not within 2s, the loop is deadlocked", name)
		}
	}
}
