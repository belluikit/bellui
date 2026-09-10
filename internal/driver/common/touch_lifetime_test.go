package common_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// doubleTappableRect records the taps dispatched to it. It is deliberately a
// bare object rather than a widget: the point is who gets called and when, not
// what a renderer does with it.
// Counters are atomic: the dispatch lands on the driver's goroutine, so a plain
// int would be read here while it is written there.
type doubleTappableRect struct {
	canvas.Rectangle

	taps       atomic.Int32
	doubleTaps atomic.Int32
}

func (d *doubleTappableRect) Tapped(*event.PointEvent)       { d.taps.Add(1) }
func (d *doubleTappableRect) DoubleTapped(*event.PointEvent) { d.doubleTaps.Add(1) }

// lifetimeFinder is the ObjectFinder a driver supplies. canvas is settable to
// nil so a test can simulate the canvas going away underneath a pending tap.
type lifetimeFinder struct{ canvas core.Canvas }

func (f *lifetimeFinder) FindObjectAtPositionMatching(pos geo.Position,
	match func(core.CanvasObject) bool,
) (core.CanvasObject, geo.Position, int) {
	if f.canvas == nil {
		return nil, geo.NewPos(0, 0), 0
	}
	return intdriver.FindObjectAtPositionMatching(pos, match, f.canvas.Overlays().List(), f.canvas.Content())
}

func (f *lifetimeFinder) CanvasForTouch() core.Canvas { return f.canvas }

// tapOnce drives a full press-and-release on obj, which is what starts the
// double-tap wait.
func tapOnce(h *common.TouchHandler, pos geo.Position) {
	h.TapDown(pos, 0)
	h.TapUp(
		pos, 0,
		func(wid event.Tappable, ev *event.PointEvent) { wid.Tapped(ev) },
		func(wid event.SecondaryTappable, ev *event.PointEvent) {},
		func(wid event.LongPressable, ev *event.PointEvent) {},
		func(wid event.DoubleTappable, ev *event.PointEvent) { wid.DoubleTapped(ev) },
		func(wid event.Draggable, ev *event.DragEvent) {},
	)
}

func newTapHarness(t *testing.T) (*common.TouchHandler, *lifetimeFinder, *doubleTappableRect) {
	t.Helper()
	test.NewTempApp(t)

	target := &doubleTappableRect{}
	target.SetMinSize(geo.NewSquareSize(100))
	c := test.NewCanvas()
	c.SetPadded(false)
	c.SetContent(container.NewStack(target))
	c.Resize(geo.NewSquareSize(100))

	finder := &lifetimeFinder{canvas: c}
	h := &common.TouchHandler{}
	h.InitTouchHandler(finder)
	return h, finder, target
}

// TestPendingTap_CanceledOnTeardown pins #310: a single tap leaves a timer
// waiting TapDoubleDelay for a second one, and nothing used to stop it when the
// canvas went away. It then called Tapped on a widget whose renderer had been
// destroyed — a nil dereference that took the process down.
func TestPendingTap_CanceledOnTeardown(t *testing.T) {
	h, _, target := newTapHarness(t)

	tapOnce(h, geo.NewPos(50, 50))
	require.Zero(t, target.taps.Load(), "the tap must not dispatch before the double-tap window closes")

	h.CancelPendingTap() // what a window Close now does

	time.Sleep(common.TapDoubleDelay + 250*time.Millisecond)
	bellui.DoAndWait(func() {}) // drain anything the timer queued

	assert.Zero(t, target.taps.Load(), "an abandoned tap must not dispatch")
	assert.Zero(t, target.doubleTaps.Load(), "an abandoned tap must not dispatch")
}

// TestPendingTap_NotDispatchedAfterTargetLeavesTheTree covers the half no
// cancel can reach: the canvas outlives the gesture but the object does not —
// a dialog dismissed inside the double-tap window. Nothing signals the handler,
// so the queued callback has to check the target is still there.
func TestPendingTap_NotDispatchedAfterTargetLeavesTheTree(t *testing.T) {
	h, finder, target := newTapHarness(t)

	tapOnce(h, geo.NewPos(50, 50))

	// The widget is dismissed while the tap is still pending.
	finder.canvas.(interface{ SetContent(core.CanvasObject) }).SetContent(canvas.NewRectangle(nil))

	time.Sleep(common.TapDoubleDelay + 250*time.Millisecond)
	bellui.DoAndWait(func() {})

	assert.Zero(t, target.taps.Load(), "a tap must not dispatch to an object that has left the canvas")
	assert.Zero(t, target.doubleTaps.Load(), "a tap must not dispatch to an object that has left the canvas")
}

// TestPendingTap_StillDispatchesNormally is the control: without a teardown the
// tap must still arrive, or the guards above would be indistinguishable from
// breaking tap handling outright.
func TestPendingTap_StillDispatchesNormally(t *testing.T) {
	h, _, target := newTapHarness(t)

	tapOnce(h, geo.NewPos(50, 50))

	time.Sleep(common.TapDoubleDelay + 250*time.Millisecond)
	bellui.DoAndWait(func() {})

	assert.Equal(t, int32(1), target.taps.Load(), "an undisturbed single tap must dispatch")
	assert.Zero(t, target.doubleTaps.Load())
}
