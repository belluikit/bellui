package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
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

// A pinch's fingers are released from the contact count when it ends, and a
// fresh down on an id forgets what it was spent on. Platforms do lose
// releases — the iOS driver renumbers a surviving touch whenever another one
// ends, and Android reports ACTION_CANCEL as a move — and a contact left down
// for good must not pair with every later finger as a pinch, nor swallow the
// taps of a later contact that reuses its id.
func TestTouchHandler_LostPinchReleaseHeals(t *testing.T) {
	target := &pinchRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)

	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(200, 100), 2)
	require.NotNil(t, h.pinch)
	pinchTapUp(h, geo.NewPos(100, 100), 1) // finger 2's release never arrives
	assert.Equal(t, 1, target.ended)

	// A later single finger is not the second of two.
	h.TapDown(geo.NewPos(150, 150), 3)
	assert.Nil(t, h.pinch, "a finger lost by the platform must not start a pinch")
	h.TapMove(geo.NewPos(300, 150), 3, nil)
	assert.Empty(t, target.pinches)
	pinchTapUp(h, geo.NewPos(300, 150), 3)
	assert.Equal(t, 1, target.taps, "the later finger taps")

	// A new contact on the lost finger's id taps like any other.
	h.TapDown(geo.NewPos(100, 100), 2)
	pinchTapUp(h, geo.NewPos(100, 100), 2)
	assert.Equal(t, 2, target.taps, "a reused id is a new contact, not a spent one")
	assert.Empty(t, h.contacts)
}

// The finger that outlives a pinch is spent: one landing beside it starts no
// second pinch, and taps normally.
func TestTouchHandler_PinchSurvivorDoesNotPinchAgain(t *testing.T) {
	target := &pinchRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(200, 100), 2)
	pinchTapUp(h, geo.NewPos(100, 100), 1)

	h.TapDown(geo.NewPos(150, 100), 3) // finger 2 is still down
	assert.Nil(t, h.pinch)
	pinchTapUp(h, geo.NewPos(200, 100), 2)
	pinchTapUp(h, geo.NewPos(150, 100), 3)
	assert.Equal(t, 1, target.taps, "only the new finger taps")
	assert.Equal(t, 1, target.ended)
}

// Fingers that land on the same point have no distance to scale from; the
// pinch measures from the first move instead of ignoring every move.
func TestTouchHandler_PinchFromCoincidentFingers(t *testing.T) {
	target := &pinchRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(100, 100), 2)
	h.TapMove(geo.NewPos(200, 100), 2, nil)
	h.TapMove(geo.NewPos(300, 100), 2, nil)
	require.Len(t, target.pinches, 2)
	assert.InDelta(t, 1.0, target.pinches[0].Scale, 1e-6)
	assert.InDelta(t, 2.0, target.pinches[1].Scale, 1e-6, "from 100 to 200 apart")
}

// A move that keeps the distance still reports where the fingers are, so a
// widget can pan with them.
func TestTouchHandler_PinchReportsFocusAtEqualDistance(t *testing.T) {
	target := &pinchRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(200, 100), 2)
	h.TapMove(geo.NewPos(100, 200), 2, nil) // still 100 from finger 1
	require.Len(t, target.pinches, 1)
	assert.InDelta(t, 1.0, target.pinches[0].Scale, 1e-6)
	assert.Equal(t, geo.NewPos(100, 150), target.pinches[0].AbsolutePosition)
}

// pinchTouchRect is a Touchable Pinchable, for the first finger's touch.
type pinchTouchRect struct {
	pinchRect
	downs, ups, cancels int
}

func (p *pinchTouchRect) TouchDown(*mobile.TouchEvent)   { p.downs++ }
func (p *pinchTouchRect) TouchUp(*mobile.TouchEvent)     { p.ups++ }
func (p *pinchTouchRect) TouchCancel(*mobile.TouchEvent) { p.cancels++ }

// The first finger's Touchable is told its touch is over when the finger
// becomes half of a pinch, as TapCancel would tell it.
func TestTouchHandler_PinchCancelsFirstFingersTouch(t *testing.T) {
	target := &pinchTouchRect{pinchRect: pinchRect{Rectangle: canvas.NewRectangle(color.Black)}}
	h := newPinchHandler(target)
	h.TapDown(geo.NewPos(100, 100), 1)
	require.Equal(t, 1, target.downs)
	h.TapDown(geo.NewPos(200, 100), 2)
	assert.Equal(t, 1, target.cancels)
	pinchTapUp(h, geo.NewPos(100, 100), 1)
	pinchTapUp(h, geo.NewPos(200, 100), 2)
	assert.Equal(t, 1, target.cancels)
	assert.Zero(t, target.ups)
}

// When both of a pinch's releases are lost, the next down on either id ends
// that pinch rather than driving it against a finger that is long gone.
func TestTouchHandler_DownOnPinchFingerEndsLostPinch(t *testing.T) {
	target := &pinchRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(200, 100), 2) // neither release arrives

	h.TapDown(geo.NewPos(150, 150), 1)
	assert.Equal(t, 1, target.ended, "the lost pinch ends")
	assert.Nil(t, h.pinch)
	h.TapMove(geo.NewPos(400, 150), 1, nil)
	assert.Empty(t, target.pinches, "the new finger does not pinch against the lost one")
	pinchTapUp(h, geo.NewPos(400, 150), 1)
	assert.Equal(t, 1, target.taps)
	assert.Equal(t, 1, target.ended)
}
