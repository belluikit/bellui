//go:build !windows || !ci

package mobile

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/mobile/event/touch"
	"github.com/belluikit/bellui/widget"
)

// middle is the middle of a 200x200 canvas in device pixels, which is where
// touch events are reported: 100 dp times the canvas's scale, which differs
// between platforms (2 on macOS, 1 on Linux CI).
func middle(w *window) float32 { return 100 * w.canvas.Scale() }

// A touch the system cancels (Android ACTION_CANCEL, iOS touchesCancelled)
// ends without tapping (#392). It used to arrive as a move on Android, so its
// release was never seen, and as an ordinary end on iOS, so it could tap.
func Test_driver_CancelledTouchDoesNotTap(t *testing.T) {
	d := &driver{}
	w := d.CreateWindow("cancel").(*window)
	var taps atomic.Int32
	w.SetContent(widget.NewButton("tap me", func() { taps.Add(1) }))
	w.Show()
	w.canvas.Resize(geo.NewSize(200, 200))
	m := middle(w)

	d.handleTouch(w, touch.Event{X: m, Y: m, Sequence: 0, Type: touch.TypeBegin})
	d.handleTouch(w, touch.Event{X: m, Y: m, Sequence: 0, Type: touch.TypeCancel})
	time.Sleep(600 * time.Millisecond)
	assert.Zero(t, taps.Load(), "a cancelled touch must not tap")
	assert.Empty(t, d.touchWindows, "the cancelled touch is forgotten")

	// The same touch ended normally does tap: the check above can fail.
	d.handleTouch(w, touch.Event{X: m, Y: m, Sequence: 0, Type: touch.TypeBegin})
	d.handleTouch(w, touch.Event{X: m, Y: m, Sequence: 0, Type: touch.TypeEnd})
	assert.Eventually(t, func() bool { return taps.Load() == 1 }, time.Second, 10*time.Millisecond)
}

// A touch keeps the window it began in: if another window becomes current
// while the finger is down, the release still reaches the first one (#392).
func Test_driver_TouchKeepsItsWindow(t *testing.T) {
	d := &driver{}
	first := d.CreateWindow("first").(*window)
	var taps atomic.Int32
	first.SetContent(widget.NewButton("first", func() { taps.Add(1) }))
	first.Show()
	first.canvas.Resize(geo.NewSize(200, 200))
	m := middle(first)

	d.handleTouch(first, touch.Event{X: m, Y: m, Sequence: 3, Type: touch.TypeBegin})
	second := d.CreateWindow("second").(*window)
	second.SetContent(widget.NewLabel("second"))
	second.Show()

	d.handleTouch(second, touch.Event{X: m, Y: m, Sequence: 3, Type: touch.TypeEnd})
	assert.Eventually(t, func() bool { return taps.Load() == 1 }, time.Second, 10*time.Millisecond,
		"the release reaches the window the touch began in")
}
