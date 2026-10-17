//go:build !windows || !ci

package mobile

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/mobile/event/touch"
	"github.com/belluikit/bellui/widget"
)

// middle is the middle of a 200x200 canvas in device pixels, which is where
// touch events are reported: 100 dp times the canvas's scale, which differs
// between platforms (2 on macOS, 1 on Linux CI).
func middle(w *window) float32 { return 100 * w.canvas.Scale() }

// buttonWindow shows a window filled by a button that counts its taps. A
// Button is not DoubleTappable, so a release that taps it does so before
// TapUp returns: no test here has to wait to see that a tap did not happen.
func buttonWindow(t *testing.T, d *driver, title string) (*window, *atomic.Int32) {
	t.Helper()
	w := d.CreateWindow(title).(*window)
	var taps atomic.Int32
	w.SetContent(widget.NewButton(title, func() { taps.Add(1) }))
	w.Show()
	w.canvas.Resize(geo.NewSize(200, 200))
	return w, &taps
}

func closeOnCleanup(t *testing.T, ws ...*window) {
	t.Cleanup(func() {
		for _, w := range ws {
			w.Close()
		}
	})
}

// A touch the system cancels (Android ACTION_CANCEL, iOS touchesCancelled)
// ends without tapping (#392). It used to arrive as a move on Android, so its
// release was never seen, and as an ordinary end on iOS, so it could tap.
func Test_driver_CancelledTouchDoesNotTap(t *testing.T) {
	d := &driver{}
	w, taps := buttonWindow(t, d, "cancel")
	closeOnCleanup(t, w)
	m := middle(w)

	d.handleTouch(w, touch.Event{X: m, Y: m, Sequence: 0, Type: touch.TypeBegin})
	d.handleTouch(w, touch.Event{X: m, Y: m, Sequence: 0, Type: touch.TypeCancel})
	assert.Zero(t, taps.Load(), "a cancelled touch must not tap")
	assert.Empty(t, d.touchWindows, "the cancelled touch is forgotten")

	// The same touch ended normally does tap, and at once: the check above
	// can fail.
	d.handleTouch(w, touch.Event{X: m, Y: m, Sequence: 0, Type: touch.TypeBegin})
	d.handleTouch(w, touch.Event{X: m, Y: m, Sequence: 0, Type: touch.TypeEnd})
	assert.Equal(t, int32(1), taps.Load())
}

// A touch keeps the window it began in: if another window becomes current
// while the finger is down, the release still reaches the first one (#392).
func Test_driver_TouchKeepsItsWindow(t *testing.T) {
	d := &driver{}
	first, taps := buttonWindow(t, d, "first")
	m := middle(first)

	d.handleTouch(first, touch.Event{X: m, Y: m, Sequence: 3, Type: touch.TypeBegin})
	second, secondTaps := buttonWindow(t, d, "second")
	closeOnCleanup(t, second, first)

	d.handleTouch(second, touch.Event{X: m, Y: m, Sequence: 3, Type: touch.TypeEnd})
	assert.Equal(t, int32(1), taps.Load(), "the release reaches the window the touch began in")
	assert.Zero(t, secondTaps.Load())
}

// A touch whose window closes while the finger is down is cancelled there,
// and the rest of it goes nowhere. Its release used to be delivered to the
// closed window, whose content is still in place: the hit test found the
// button the finger went down on, and it tapped in a window nobody could see.
func Test_driver_TouchInAClosedWindowDoesNotTap(t *testing.T) {
	d := &driver{}
	under := d.CreateWindow("under").(*window)
	rec := newCancelRecorder()
	under.SetContent(rec)
	under.Show()
	under.canvas.Resize(geo.NewSize(200, 200))
	closing, taps := buttonWindow(t, d, "closing")
	closeOnCleanup(t, under)
	m := middle(closing)

	d.handleTouch(closing, touch.Event{X: m, Y: m, Sequence: 1, Type: touch.TypeBegin})
	closing.Close()
	d.handleTouch(under, touch.Event{X: m + 30, Y: m, Sequence: 1, Type: touch.TypeMove})
	d.handleTouch(under, touch.Event{X: m + 30, Y: m, Sequence: 1, Type: touch.TypeEnd})
	assert.Zero(t, taps.Load(), "nothing taps in a closed window")
	assert.Zero(t, rec.dragged+rec.tapped, "nor drags or taps in the window that never saw the touch go down")
	assert.Empty(t, d.touchWindows, "the dropped touch is forgotten when it ends")

	// The next touch with that id is an ordinary one.
	d.handleTouch(under, touch.Event{X: m, Y: m, Sequence: 1, Type: touch.TypeBegin})
	d.handleTouch(under, touch.Event{X: m, Y: m, Sequence: 1, Type: touch.TypeEnd})
	assert.Equal(t, 1, rec.tapped)
}

// Under an explore-by-touch screen reader a qualified passthrough stream is a
// touch like any other: it keeps the window it began in (#392).
func Test_driver_ExploreTouchKeepsItsWindow(t *testing.T) {
	d := &driver{}
	first, taps := buttonWindow(t, d, "first")
	m := middle(first)

	d.exploreTouch(first, touch.Event{X: m, Y: m, Sequence: 0, Type: touch.TypeBegin})
	d.tbHoldQualify(d.tbPass.gen) // held long enough: the stream is dispatched
	second, secondTaps := buttonWindow(t, d, "second")
	closeOnCleanup(t, second, first)

	d.exploreTouch(second, touch.Event{X: m, Y: m, Sequence: 0, Type: touch.TypeEnd})
	assert.Zero(t, secondTaps.Load(), "the window the stream did not begin in")
	assert.Equal(t, int32(1), taps.Load(), "the release reaches the window the stream began in")
}

// A passthrough stream whose window closes is cancelled there too.
func Test_driver_ExploreTouchInAClosedWindowDoesNotTap(t *testing.T) {
	d := &driver{}
	under, underTaps := buttonWindow(t, d, "under")
	closing, taps := buttonWindow(t, d, "closing")
	closeOnCleanup(t, under)
	m := middle(closing)

	d.exploreTouch(closing, touch.Event{X: m, Y: m, Sequence: 0, Type: touch.TypeBegin})
	d.tbHoldQualify(d.tbPass.gen)
	closing.Close()
	d.exploreTouch(under, touch.Event{X: m, Y: m, Sequence: 0, Type: touch.TypeEnd})
	assert.Zero(t, taps.Load())
	assert.Zero(t, underTaps.Load())
	assert.Nil(t, d.tbPass.win, "the stream no longer holds the closed window")
}
