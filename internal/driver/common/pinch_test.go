package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// pinchRect records pinches and taps.
type pinchRect struct {
	*canvas.Rectangle
	pinches []event.PinchEvent
	ended   int
	taps    int
}

func (p *pinchRect) Pinched(e *event.PinchEvent) { p.pinches = append(p.pinches, *e) }
func (p *pinchRect) PinchEnded()                 { p.ended++ }
func (p *pinchRect) Tapped(*event.PointEvent)    { p.taps++ }

func newPinchHandler(objects ...core.CanvasObject) *TouchHandler {
	h := &TouchHandler{}
	h.InitTouchHandler(&touchScrollFinder{objects: objects})
	return h
}

func pinchTapUp(h *TouchHandler, pos geo.Position, id int) {
	h.TapUp(pos, id, func(w event.Tappable, e *event.PointEvent) { w.Tapped(e) }, nil, nil, nil,
		func(w event.Draggable, _ *event.DragEvent) { w.DragEnd() })
}

// Two fingers on a Pinchable pinch it: a move reports the change in finger
// distance and the midpoint, either finger lifting ends it once, and neither
// finger taps.
func TestTouchHandler_TwoFingersPinch(t *testing.T) {
	target := &pinchRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)

	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(200, 100), 2) // fingers 100 apart
	h.TapMove(geo.NewPos(300, 100), 2, nil)
	require.Len(t, target.pinches, 1)
	assert.InDelta(t, 2.0, target.pinches[0].Scale, 1e-6, "fingers moved from 100 to 200 apart")
	assert.Equal(t, geo.NewPos(200, 100), target.pinches[0].AbsolutePosition, "focus is the midpoint")

	h.TapMove(geo.NewPos(250, 100), 2, nil) // 200 -> 150 apart
	require.Len(t, target.pinches, 2)
	assert.InDelta(t, 0.75, target.pinches[1].Scale, 1e-6)

	pinchTapUp(h, geo.NewPos(250, 100), 2)
	pinchTapUp(h, geo.NewPos(100, 100), 1)
	assert.Equal(t, 1, target.ended, "PinchEnded exactly once")
	assert.Zero(t, target.taps, "a pinch's fingers must not tap")

	// The handler is back to normal: a single finger taps again.
	h.TapDown(geo.NewPos(100, 100), 3)
	pinchTapUp(h, geo.NewPos(100, 100), 3)
	assert.Equal(t, 1, target.taps)
}

// Without a Pinchable under the fingers, a second finger changes nothing.
func TestTouchHandler_SecondFingerWithoutPinchable(t *testing.T) {
	target := &tappableRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(200, 100), 2)
	assert.Nil(t, h.pinch)
	assert.Empty(t, h.pinchSpent)
}
