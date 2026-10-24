//go:build (ci || android || ios || mobile) && (!windows || !ci) && !wasm

// The Scroll is draggable only where internal/widget/scroller_mobile.go is
// built, so this file carries that file's constraint (#537).

package mobile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bcanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver/mobile/event/touch"
	"github.com/belluikit/bellui/widget"
)

// Only a Draggable that asks for momentum (event.MomentumDraggable) coasts
// after a touch release (#751): a Slider stops where the finger lifted, as on
// desktop, while a Scroll still flings — from the finger's speed at release,
// so a drag held still before lifting does not coast.

// momentumWindow is a 200x200 window filled by content, on a driver that
// queues to the test app's main loop, as the real one does.
func momentumWindow(t *testing.T, content core.CanvasObject) (*driver, *window) {
	t.Helper()
	cd := &driver{queuedFuncs: d.queuedFuncs}
	var w *window
	onMain(func() {
		w = cd.CreateWindow("momentum").(*window)
		w.SetContent(content)
		w.Show()
		w.canvas.Resize(geo.NewSize(200, 200))
	})
	t.Cleanup(func() { onMain(w.Close) })
	return cd, w
}

// touchAt sends one touch event at canvas position (x, y).
func touchAt(cd *driver, w *window, x, y float32, typ touch.Type) {
	s := w.canvas.Scale()
	cd.handleTouch(w, touch.Event{X: x * s, Y: y * s, Type: typ})
}

func Test_driver_SliderStopsWhereTheFingerLifts(t *testing.T) {
	slider := widget.NewSlider(0, 100)
	ended := 0
	slider.OnChangeEnded = func(float64) { ended++ }
	cd, w := momentumWindow(t, slider)

	var atRelease float64
	onMain(func() {
		// A quick flick: 40pt moves, still moving when it lets go.
		touchAt(cd, w, 20, 100, touch.TypeBegin)
		for x := float32(60); x <= 140; x += 40 {
			touchAt(cd, w, x, 100, touch.TypeMove)
		}
		touchAt(cd, w, 180, 100, touch.TypeEnd)
		atRelease = slider.Value
		assert.False(t, slider.Dragging(), "the release ends the drag at once: no coast")
		assert.Equal(t, 1, ended, "OnChangeEnded fires on release, not after a coast")
	})
	time.Sleep(300 * time.Millisecond) // well past where a coast would have ended it
	onMain(func() {
		assert.Equal(t, atRelease, slider.Value, "the value stays where the finger lifted")
		assert.Equal(t, 1, ended)
	})
}

// newMomentumScroll is a vertical Scroll over content far taller than the
// window.
func newMomentumScroll() *container.Scroll {
	tall := bcanvas.NewRectangle(nil)
	tall.SetMinSize(geo.NewSize(150, 4000))
	return container.NewVScroll(tall)
}

func Test_driver_ScrollFlingsAfterAFastSwipe(t *testing.T) {
	scroll := newMomentumScroll()
	cd, w := momentumWindow(t, scroll)

	onMain(func() { touchAt(cd, w, 50, 180, touch.TypeBegin) })
	for y := float32(150); y > 60; y -= 30 {
		time.Sleep(10 * time.Millisecond)
		onMain(func() { touchAt(cd, w, 50, y, touch.TypeMove) })
	}
	var atRelease float32
	onMain(func() {
		touchAt(cd, w, 50, 30, touch.TypeMove)
		touchAt(cd, w, 50, 10, touch.TypeEnd) // still moving as it lets go
		atRelease = scroll.Offset.Y
	})
	require.Positive(t, atRelease)
	time.Sleep(200 * time.Millisecond)
	onMain(func() {
		assert.Greater(t, scroll.Offset.Y, atRelease, "the content flies on past the release")
	})
}

func Test_driver_ScrollHeldStillBeforeLiftDoesNotCoast(t *testing.T) {
	scroll := newMomentumScroll()
	cd, w := momentumWindow(t, scroll)

	onMain(func() {
		touchAt(cd, w, 50, 180, touch.TypeBegin)
		touchAt(cd, w, 50, 140, touch.TypeMove)
		touchAt(cd, w, 50, 100, touch.TypeMove) // a 40pt last move
	})
	time.Sleep(150 * time.Millisecond) // held still
	var atRelease float32
	onMain(func() {
		touchAt(cd, w, 50, 100, touch.TypeEnd)
		atRelease = scroll.Offset.Y
	})
	require.Positive(t, atRelease)
	time.Sleep(300 * time.Millisecond)
	onMain(func() {
		assert.Equal(t, atRelease, scroll.Offset.Y, "a finger held still before lifting does not fling")
	})
}
