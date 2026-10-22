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

// Without a Pinchable under the fingers, two fingers moving together pan, and
// neither is spent once they lift.
func TestTouchHandler_SecondFingerWithoutPinchable(t *testing.T) {
	target := &tappableRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	var scrolls []geo.Delta
	h.ScrollCallback = func(_ geo.Position, d geo.Delta) { scrolls = append(scrolls, d) }
	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(200, 100), 2)
	for y := float32(105); y <= 140; y += 5 { // both fingers move down together
		h.TapMove(geo.NewPos(100, y), 1, nil)
		h.TapMove(geo.NewPos(200, y), 2, nil)
	}
	require.NotEmpty(t, scrolls, "two fingers moving together without a Pinchable pan")
	pinchTapUp(h, geo.NewPos(100, 140), 1)
	pinchTapUp(h, geo.NewPos(200, 140), 2)
	assert.Empty(t, h.pinchSpent)
}

// draggableRect records drags, for the dissolve case.
type draggableRect struct {
	*canvas.Rectangle
	dragged, ended int
}

func (d *draggableRect) Dragged(*event.DragEvent) { d.dragged++ }
func (d *draggableRect) DragEnd()                 { d.ended++ }

// Without a Pinchable, a finger moving beside a resting one (a palm, a thumb)
// is not a two-finger gesture: it drags as it would alone.
func TestTouchHandler_RestingFingerDoesNotStopADrag(t *testing.T) {
	target := &draggableRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	var scrolls int
	h.ScrollCallback = func(geo.Position, geo.Delta) { scrolls++ }
	drag := func(w event.Draggable, e *event.DragEvent) { w.Dragged(e) }
	h.TapDown(geo.NewPos(21, 21), 0) // resting
	h.TapDown(geo.NewPos(100, 100), 1)
	for x := float32(110); x <= 160; x += 10 {
		h.TapMove(geo.NewPos(x, 100), 1, drag)
	}
	assert.Positive(t, target.dragged, "the moving finger drags")
	assert.Zero(t, scrolls, "and does not pan")
	assert.Nil(t, h.pinch)
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
	h.TapMove(geo.NewPos(260, 100), 2, nil) // a pinch: the fingers spread
	require.NotNil(t, h.pinch)
	pinchTapUp(h, geo.NewPos(100, 100), 1) // finger 2's release never arrives
	assert.Equal(t, 1, target.ended)

	// A later single finger is not the second of two.
	h.TapDown(geo.NewPos(150, 150), 3)
	assert.Nil(t, h.pinch, "a finger lost by the platform must not start a pinch")
	h.TapMove(geo.NewPos(300, 150), 3, nil)
	assert.Len(t, target.pinches, 1)
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
	h.TapMove(geo.NewPos(260, 100), 2, nil) // a pinch: the fingers spread
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

// Two fingers moving together over a Pinchable are a pan (#398): they scroll
// from where the gesture started, with momentum on release, and the target
// gets no Pinched — the scroll reaches it first, so a widget can pan itself or
// let the scroll pass to the page around it.
func TestTouchHandler_TwoFingersTogetherPan(t *testing.T) {
	target := &pinchRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	var origins []geo.Position
	var scrolls []geo.Delta
	var ended int
	h.ScrollCallback = func(o geo.Position, d geo.Delta) { origins = append(origins, o); scrolls = append(scrolls, d) }
	h.ScrollEndCallback = func(geo.Position, geo.Delta) { ended++ }

	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(200, 100), 2)
	for y := float32(110); y <= 160; y += 10 { // both fingers move up the screen together
		h.TapMove(geo.NewPos(100, y), 1, nil)
		h.TapMove(geo.NewPos(200, y), 2, nil)
	}
	require.NotEmpty(t, scrolls)
	var total float32
	for _, d := range scrolls {
		total += d.DY
	}
	assert.InDelta(t, 60, total, 1e-3, "the scroll follows the midpoint, 100 -> 160")
	assert.Equal(t, geo.NewPos(150, 100), origins[0], "scrolls from where the gesture started")
	assert.Empty(t, target.pinches, "moving together is not a pinch")

	pinchTapUp(h, geo.NewPos(100, 160), 1)
	pinchTapUp(h, geo.NewPos(200, 160), 2)
	assert.Equal(t, 1, ended, "the pan ends once, seeding momentum")
	assert.Zero(t, target.ended)
	assert.Zero(t, target.taps)
}

// A one-handed pinch keeps one finger still: the midpoint moves half as far as
// the distance changes, which must still read as a pinch, not a pan.
func TestTouchHandler_OneHandedPinchIsAPinch(t *testing.T) {
	target := &pinchRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	var scrolls int
	h.ScrollCallback = func(geo.Position, geo.Delta) { scrolls++ }
	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(200, 100), 2)
	for x := float32(205); x <= 260; x += 5 {
		h.TapMove(geo.NewPos(x, 100), 2, nil) // finger 1 stays put
	}
	assert.NotEmpty(t, target.pinches)
	assert.Zero(t, scrolls)
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
	h.TapDown(geo.NewPos(200, 100), 2)      // neither release arrives
	h.TapMove(geo.NewPos(260, 100), 2, nil) // a pinch: the fingers spread

	h.TapDown(geo.NewPos(150, 150), 1)
	assert.Equal(t, 1, target.ended, "the lost pinch ends")
	assert.Nil(t, h.pinch)
	h.TapMove(geo.NewPos(400, 150), 1, nil)
	assert.Len(t, target.pinches, 1, "the new finger does not pinch against the lost one")
	pinchTapUp(h, geo.NewPos(400, 150), 1)
	assert.Equal(t, 1, target.taps)
	assert.Equal(t, 1, target.ended)
}

// touchTapRect is a Touchable, Tappable target with no Pinchable, for the
// tentative two-finger gesture.
type touchTapRect struct {
	*canvas.Rectangle
	taps, downs, ups, cancels int
}

func (r *touchTapRect) Tapped(*event.PointEvent)       { r.taps++ }
func (r *touchTapRect) TouchDown(*mobile.TouchEvent)   { r.downs++ }
func (r *touchTapRect) TouchUp(*mobile.TouchEvent)     { r.ups++ }
func (r *touchTapRect) TouchCancel(*mobile.TouchEvent) { r.cancels++ }

// Without a Pinchable, a finger tapping beside a resting one taps: until two
// fingers move together, each is a single contact.
func TestTouchHandler_TapBesideRestingFingerTaps(t *testing.T) {
	target := &touchTapRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	h.ScrollCallback = func(geo.Position, geo.Delta) {}
	h.TapDown(geo.NewPos(20, 20), 1) // resting
	h.TapDown(geo.NewPos(100, 100), 2)
	assert.Equal(t, 2, target.downs, "the second finger touches down too")
	pinchTapUp(h, geo.NewPos(100, 100), 2)
	assert.Equal(t, 1, target.taps, "the second finger taps")
	assert.Zero(t, target.cancels)

	h.TapDown(geo.NewPos(100, 100), 3) // and again, still beside the resting finger
	pinchTapUp(h, geo.NewPos(100, 100), 3)
	assert.Equal(t, 2, target.taps)
}

// When two fingers without a Pinchable start to pan, what each had started on
// its own is cancelled, and neither taps on release.
func TestTouchHandler_PanTakesSingleContacts(t *testing.T) {
	target := &touchTapRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	var scrolls int
	h.ScrollCallback = func(geo.Position, geo.Delta) { scrolls++ }
	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(200, 100), 2)
	for y := float32(110); y <= 150; y += 10 {
		h.TapMove(geo.NewPos(100, y), 1, nil)
		h.TapMove(geo.NewPos(200, y), 2, nil)
	}
	require.Positive(t, scrolls)
	assert.Equal(t, 2, target.cancels, "both touches end when the pan takes them")
	pinchTapUp(h, geo.NewPos(100, 150), 1)
	pinchTapUp(h, geo.NewPos(200, 150), 2)
	assert.Zero(t, target.taps)
	assert.Zero(t, target.ups)
	assert.Empty(t, h.pinchSpent)
}

// A fast pan moves each finger past the slop in one move. The first finger's
// move alone changes the distance as much as it moves the midpoint, so the
// decision waits for the second finger's: side by side over a Pinchable, that
// is a pan, not a pinch.
func TestTouchHandler_FastPanIsNotAPinch(t *testing.T) {
	target := &pinchRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	var scrolls []geo.Delta
	h.ScrollCallback = func(_ geo.Position, d geo.Delta) { scrolls = append(scrolls, d) }
	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(200, 100), 2)
	for x := float32(20); x <= 80; x += 20 {
		h.TapMove(geo.NewPos(100+x, 100), 1, nil)
		h.TapMove(geo.NewPos(200+x, 100), 2, nil)
	}
	assert.Empty(t, target.pinches, "fingers moving together sideways do not pinch")
	var total float32
	for _, d := range scrolls {
		total += d.DX
	}
	assert.InDelta(t, 80, total, 1e-3, "the pan follows the midpoint")
}

// Likewise without a Pinchable: a fast flick must pan rather than dissolve
// into a one-finger drag because the second finger had not reported yet.
func TestTouchHandler_FastFlickWithoutPinchablePans(t *testing.T) {
	target := &draggableRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	var scrolls int
	h.ScrollCallback = func(geo.Position, geo.Delta) { scrolls++ }
	drag := func(w event.Draggable, e *event.DragEvent) { w.Dragged(e) }
	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(200, 100), 2)
	for y := float32(130); y <= 190; y += 30 {
		h.TapMove(geo.NewPos(100, y), 1, drag)
		h.TapMove(geo.NewPos(200, y), 2, drag)
	}
	assert.Positive(t, scrolls)
	assert.Zero(t, target.dragged, "a two-finger flick is not a drag")
}

// Momentum is seeded with a frame's motion — both fingers' steps — as a
// one-finger flick's is with its last move.
func TestTouchHandler_PanMomentumIsTheFramesMotion(t *testing.T) {
	target := &pinchRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	h.ScrollCallback = func(geo.Position, geo.Delta) {}
	var seeds []geo.Delta
	h.ScrollEndCallback = func(_ geo.Position, d geo.Delta) { seeds = append(seeds, d) }
	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(200, 100), 2)
	for y := float32(106); y <= 160; y += 6 { // 6 px a frame
		h.TapMove(geo.NewPos(100, y), 1, nil)
		h.TapMove(geo.NewPos(200, y), 2, nil)
	}
	pinchTapUp(h, geo.NewPos(100, 160), 1)
	pinchTapUp(h, geo.NewPos(200, 160), 2)
	require.Len(t, seeds, 1)
	assert.InDelta(t, 6, seeds[0].DY, 1e-3)
}

// A pan the system cancels, or whose release was lost, ends where it stands:
// no momentum, as for a one-finger scroll (ScrollEndCallback).
func TestTouchHandler_PanEndedWithoutReleaseHasNoMomentum(t *testing.T) {
	for _, tc := range []struct {
		name string
		end  func(h *TouchHandler)
	}{
		{"cancel", func(h *TouchHandler) { h.TapCancel(geo.NewPos(100, 150), 1) }},
		{"lost release", func(h *TouchHandler) { h.TapDown(geo.NewPos(300, 300), 1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := &pinchRect{Rectangle: canvas.NewRectangle(color.Black)}
			h := newPinchHandler(target)
			var scrolls, ended int
			h.ScrollCallback = func(geo.Position, geo.Delta) { scrolls++ }
			h.ScrollEndCallback = func(geo.Position, geo.Delta) { ended++ }
			h.TapDown(geo.NewPos(100, 100), 1)
			h.TapDown(geo.NewPos(200, 100), 2)
			for y := float32(110); y <= 150; y += 10 {
				h.TapMove(geo.NewPos(100, y), 1, nil)
				h.TapMove(geo.NewPos(200, y), 2, nil)
			}
			require.Positive(t, scrolls)
			tc.end(h)
			assert.Nil(t, h.pinch)
			assert.Zero(t, ended)
		})
	}
}

// A driver that cannot pan (no ScrollCallback) keeps the pinch as it was: two
// fingers over a Pinchable pinch from their first move, and a move that keeps
// the distance reports the new focus, so a widget can pan with them. Without a
// Pinchable, they form no gesture.
func TestTouchHandler_NoScrollCallbackPinchesFromFirstMove(t *testing.T) {
	target := &pinchRect{Rectangle: canvas.NewRectangle(color.Black)}
	h := newPinchHandler(target)
	h.TapDown(geo.NewPos(100, 100), 1)
	h.TapDown(geo.NewPos(200, 100), 2)
	h.TapMove(geo.NewPos(100, 200), 2, nil) // still 100 from finger 1
	require.Len(t, target.pinches, 1)
	assert.InDelta(t, 1.0, target.pinches[0].Scale, 1e-6)
	assert.Equal(t, geo.NewPos(100, 150), target.pinches[0].AbsolutePosition)

	other := &touchTapRect{Rectangle: canvas.NewRectangle(color.Black)}
	h = newPinchHandler(other)
	h.TapDown(geo.NewPos(20, 20), 1)
	h.TapDown(geo.NewPos(100, 100), 2)
	assert.Nil(t, h.pinch)
	pinchTapUp(h, geo.NewPos(100, 100), 2)
	assert.Equal(t, 1, other.taps)
}
