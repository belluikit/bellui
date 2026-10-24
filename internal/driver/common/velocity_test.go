package common

import (
	"image/color"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// A released drag or scroll coasts from the finger's speed at release, not
// from the last move event (#751): #756 measured a drag whose last move was
// 40pt, held still before lifting, coasting for about half a second.

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time          { return c.t }
func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

// useFakeClock makes h stamp its velocity samples with a clock the test moves.
func useFakeClock(h *TouchHandler) *fakeClock {
	clk := &fakeClock{t: time.Unix(1000, 0)}
	h.now = clk.now
	return clk
}

// releaseDrag lifts tapID at pos and returns the release's DragEvent.
func releaseDrag(t *testing.T, h *TouchHandler, pos geo.Position, tapID int) *event.DragEvent {
	t.Helper()
	var got *event.DragEvent
	h.TapUp(pos, tapID, func(event.Tappable, *event.PointEvent) {},
		func(event.SecondaryTappable, *event.PointEvent) {},
		func(event.LongPressable, *event.PointEvent) {},
		func(event.DoubleTappable, *event.PointEvent) {},
		func(_ event.Draggable, ev *event.DragEvent) { got = ev })
	require.NotNil(t, got, "the release ends a drag")
	return got
}

func newVelocityDragHandler() (*TouchHandler, *fakeClock) {
	target := &draggableScrollableRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := &TouchHandler{}
	h.InitTouchHandler(&touchScrollFinder{objects: []core.CanvasObject{target}})
	return h, useFakeClock(h)
}

func TestTouchHandler_DragReleaseSeedIsTheReleaseSpeed(t *testing.T) {
	h, clk := newVelocityDragHandler()
	h.TapDown(geo.NewPos(10, 50), 1)
	for x := float32(20); x <= 60; x += 10 { // 10pt every 16ms
		clk.advance(coastStep)
		h.TapMove(geo.NewPos(x, 50), 1, noopDrag)
	}
	ev := releaseDrag(t, h, geo.NewPos(60, 50), 1)
	assert.InDelta(t, 10, ev.Dragged.DX, 1e-3, "the distance the finger covered per 16ms step")
	assert.Zero(t, ev.Dragged.DY)
}

func TestTouchHandler_DragHeldStillBeforeLiftHasNoSeed(t *testing.T) {
	h, clk := newVelocityDragHandler()
	h.TapDown(geo.NewPos(10, 50), 1)
	clk.advance(coastStep)
	h.TapMove(geo.NewPos(50, 50), 1, noopDrag) // a 40pt last move...
	clk.advance(300 * time.Millisecond)        // ...then held still
	ev := releaseDrag(t, h, geo.NewPos(50, 50), 1)
	assert.Equal(t, geo.Delta{}, ev.Dragged, "a finger held still before lifting gives no momentum")
}

func TestTouchHandler_DragStoppedBrieflyBeforeLiftHasNoSeed(t *testing.T) {
	h, clk := newVelocityDragHandler()
	h.TapDown(geo.NewPos(10, 50), 1)
	for x := float32(30); x <= 90; x += 20 {
		clk.advance(coastStep)
		h.TapMove(geo.NewPos(x, 50), 1, noopDrag)
	}
	clk.advance(velocityStopped + time.Millisecond) // inside the window, but no move since
	ev := releaseDrag(t, h, geo.NewPos(90, 50), 1)
	assert.Equal(t, geo.Delta{}, ev.Dragged)
}

func TestTouchHandler_DragSeedReadsOnlyTheRecentMoves(t *testing.T) {
	h, clk := newVelocityDragHandler()
	h.TapDown(geo.NewPos(0, 50), 1)
	clk.advance(coastStep)
	h.TapMove(geo.NewPos(100, 50), 1, noopDrag) // fast, but long before the release
	for i := 0; i < 10; i++ {                   // then 2pt every 16ms for 160ms
		clk.advance(coastStep)
		h.TapMove(geo.NewPos(102+float32(i)*2, 50), 1, noopDrag)
	}
	ev := releaseDrag(t, h, geo.NewPos(120, 50), 1)
	assert.InDelta(t, 2, ev.Dragged.DX, 1e-3, "the speed over the last 100ms, not since the touch went down")
}

func TestTouchHandler_DragBurstIsNotScaledUp(t *testing.T) {
	h, clk := newVelocityDragHandler()
	h.TapDown(geo.NewPos(10, 50), 1)
	clk.advance(coastStep)
	h.TapMove(geo.NewPos(20, 50), 1, noopDrag)
	clk.advance(time.Millisecond) // two moves delivered together
	h.TapMove(geo.NewPos(30, 50), 1, noopDrag)
	ev := releaseDrag(t, h, geo.NewPos(30, 50), 1)
	assert.InDelta(t, 20*float32(coastStep)/float32(coastStep+time.Millisecond), ev.Dragged.DX, 1e-3,
		"20pt over 17ms, never more than the distance covered")
}

func TestTouchHandler_ScrollHeldStillBeforeLiftHasNoSeed(t *testing.T) {
	sc := &scrollableRect{Rectangle: canvas.NewRectangle(color.Black)}
	h, _, ends := newScrollTestHandler(sc)
	clk := useFakeClock(h)
	h.TapDown(geo.NewPos(50, 90), 1)
	clk.advance(coastStep)
	h.TapMove(geo.NewPos(50, 50), 1, noopDrag)
	clk.advance(300 * time.Millisecond)
	tapUp(h, geo.NewPos(50, 50), 1)
	require.Len(t, *ends, 1)
	assert.Equal(t, geo.Delta{}, (*ends)[0].delta)
}

func TestTouchHandler_PanHeldStillBeforeLiftHasNoSeed(t *testing.T) {
	target := &pinchRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	clk := useFakeClock(h)
	h.ScrollCallback = func(geo.Position, geo.Delta) {}
	var seeds []geo.Delta
	h.ScrollEndCallback = func(_ geo.Position, d geo.Delta) { seeds = append(seeds, d) }
	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(200, 100), 2)
	for y := float32(110); y <= 160; y += 10 {
		clk.advance(8 * time.Millisecond)
		h.TapMove(geo.NewPos(100, y), 1, nil)
		clk.advance(8 * time.Millisecond)
		h.TapMove(geo.NewPos(200, y), 2, nil)
	}
	clk.advance(300 * time.Millisecond)
	pinchTapUp(h, geo.NewPos(100, 160), 1)
	pinchTapUp(h, geo.NewPos(200, 160), 2)
	require.Len(t, seeds, 1)
	assert.Equal(t, geo.Delta{}, seeds[0], "two fingers held still before lifting give no momentum")
}
