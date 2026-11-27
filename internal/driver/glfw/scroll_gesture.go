package glfw

import (
	"math"
	"time"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/activity"
	"github.com/belluikit/bellui/internal/driver"
)

// Touchpad gestures from the pointer (#391): the phase of a touchpad scroll,
// the end of its gesture, a trackpad pinch, and ctrl+wheel as a pinch. The
// platform callbacks translate into these types, so the desktop and the web
// share them.

// gesturePhase is where a scroll or a pinch stands in a touchpad gesture, as
// the platform reports it.
type gesturePhase int

const (
	gestureNone      gesturePhase = iota // a wheel step, or no phase reported
	gestureBegan                         // the fingers started moving
	gestureChanged                       // they moved
	gestureEnded                         // they lifted
	gestureCancelled                     // the system took the gesture, or it never moved
	gestureMomentum                      // a scroll the system adds after they lifted
)

// wheelPinchRate turns a ctrl+wheel offset into a pinch scale: a wheel notch
// (an offset of 1) zooms by a tenth.
var wheelPinchRate = math.Log(1.1)

// wheelPinchEnd is how long after the last ctrl+wheel step a wheel pinch
// ends: a wheel has no lift-off to say so.
const wheelPinchEnd = 200 * time.Millisecond

// afterWheelPinchStep runs f on the main goroutine wheelPinchEnd after a
// ctrl+wheel pinch step; tests replace it.
var afterWheelPinchStep = func(f func()) {
	time.AfterFunc(wheelPinchEnd, func() { runOnMain(f) })
}

// scrollGesture is the state of the touchpad gesture under way on a window.
type scrollGesture struct {
	// began is set when a gesture began without moving: its first scroll
	// that moves is reported as its beginning instead, since a Scrollable
	// never receives a scroll without movement.
	began bool
	// enders are the ScrollEnders under the pointer when the gesture began,
	// told when it ends.
	enders []event.ScrollEnder

	// pinch is the Pinchable a trackpad pinch under way is going to, and
	// pinchOrigin its position on the canvas.
	pinch       event.Pinchable
	pinchOrigin geo.Position

	// wheelPinch is the Pinchable a ctrl+wheel pinch is going to, until
	// wheelPinchEnd passes without a step; wheelPinchSeq tells the timer of
	// an earlier step that a later one came.
	wheelPinch       event.Pinchable
	wheelPinchOrigin geo.Position
	wheelPinchSeq    int
}

// processScrollGesture handles a scroll that carries its source, phase and
// modifiers. offsets are as for processMouseScrolled.
func (w *window) processScrollGesture(xoff, yoff float64, source event.ScrollSource, phase gesturePhase, mods event.KeyModifier) {
	activity.Stamp()
	g := &w.gesture
	switch phase {
	case gestureEnded, gestureCancelled:
		w.endScrollGesture()
		return
	case gestureBegan:
		w.endScrollGesture() // an end the platform never sent
		g.began = true
		g.enders = scrollEndersAt(w.scrollablesAt(w.mousePos))
	}

	// ctrl+wheel over a Pinchable pinches it (#391): a Windows precision
	// touchpad and the browsers send a trackpad pinch that way.
	if phase == gestureNone && mods&event.KeyModifierControl != 0 && w.wheelPinched(yoff) {
		return
	}

	if math.Abs(xoff) >= scrollAccelerateCutoff {
		xoff *= scrollAccelerateRate
	}
	if math.Abs(yoff) >= scrollAccelerateCutoff {
		yoff *= scrollAccelerateRate
	}
	delta := geo.NewDelta(float32(xoff)*scrollSpeed, float32(yoff)*scrollSpeed)
	if delta == (geo.Delta{}) {
		return
	}

	p := event.ScrollPhaseUnknown
	switch phase {
	case gestureBegan, gestureChanged:
		p = event.ScrollPhaseChanged
		if g.began {
			p = event.ScrollPhaseBegan
			g.began = false
		}
	case gestureMomentum:
		p = event.ScrollPhaseMomentum
	}
	w.dispatchScrollChainPhase(w.scrollablesAt(w.mousePos), w.mousePos, delta, source, p, false)
}

// endScrollGesture tells the ScrollEnders the gesture under way began over
// that the fingers lifted.
func (w *window) endScrollGesture() {
	g := &w.gesture
	enders := g.enders
	g.began, g.enders = false, nil
	for _, e := range enders {
		e.ScrollEnded()
	}
}

// scrollEndersAt returns the ScrollEnders among a chain of scrollables.
func scrollEndersAt(chain []driver.ObjectAtPosition) []event.ScrollEnder {
	var enders []event.ScrollEnder
	for _, oap := range chain {
		if e, ok := oap.Object.(event.ScrollEnder); ok {
			enders = append(enders, e)
		}
	}
	return enders
}

// pinchableAt returns the Pinchable under pos and its position on the
// canvas, or nil.
func (w *window) pinchableAt(pos geo.Position) (event.Pinchable, geo.Position) {
	o, local, _ := w.findObjectAtPositionMatching(w.canvas, pos, func(o core.CanvasObject) bool {
		_, ok := o.(event.Pinchable)
		return ok
	})
	if o == nil {
		return nil, geo.Position{}
	}
	return o.(event.Pinchable), pos.Subtract(local)
}

// processTrackpadPinch hands a trackpad pinch step to the Pinchable under
// the pointer when the pinch began. scale is relative to the previous step.
func (w *window) processTrackpadPinch(scale float64, phase gesturePhase) {
	activity.Stamp()
	g := &w.gesture
	switch phase {
	case gestureBegan:
		if g.pinch != nil {
			g.pinch.PinchEnded() // an end the platform never sent
		}
		g.pinch, g.pinchOrigin = w.pinchableAt(w.mousePos)
	case gestureEnded, gestureCancelled:
		if g.pinch != nil {
			g.pinch.PinchEnded()
		}
		g.pinch = nil
		return
	}
	if g.pinch == nil || phase == gestureMomentum {
		return
	}
	g.pinch.Pinched(&event.PinchEvent{
		PointEvent: event.PointEvent{AbsolutePosition: w.mousePos, Position: w.mousePos.Subtract(g.pinchOrigin)},
		Scale:      float32(scale),
	})
}

// wheelPinched turns a ctrl+wheel step over a Pinchable into a pinch step,
// and reports whether there was one to pinch. The pinch ends wheelPinchEnd
// after its last step.
func (w *window) wheelPinched(yoff float64) bool {
	g := &w.gesture
	if g.wheelPinch == nil {
		g.wheelPinch, g.wheelPinchOrigin = w.pinchableAt(w.mousePos)
		if g.wheelPinch == nil {
			return false
		}
	}
	if yoff != 0 {
		g.wheelPinch.Pinched(&event.PinchEvent{
			PointEvent: event.PointEvent{AbsolutePosition: w.mousePos, Position: w.mousePos.Subtract(g.wheelPinchOrigin)},
			Scale:      float32(math.Exp(yoff * wheelPinchRate)),
		})
	}
	g.wheelPinchSeq++
	seq := g.wheelPinchSeq
	afterWheelPinchStep(func() { w.endWheelPinch(seq) })
	return true
}

// endWheelPinch ends the ctrl+wheel pinch, unless a step came after the one
// that scheduled this end.
func (w *window) endWheelPinch(seq int) {
	g := &w.gesture
	if g.wheelPinch == nil || seq != g.wheelPinchSeq {
		return
	}
	target := g.wheelPinch
	g.wheelPinch = nil
	target.PinchEnded()
}
