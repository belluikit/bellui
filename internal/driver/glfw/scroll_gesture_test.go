//go:build !no_glfw && !mobile

// No !ci, like activity_test.go: the process methods need no GLFW window, so
// this runs in the browser leg of CI too.

package glfw

import (
	"image/color"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
)

// gestureTarget is a Scrollable and a Pinchable that records what it gets;
// endsScroll makes it a ScrollEnder as well (see enderTarget).
type gestureTarget struct {
	canvas.Rectangle
	scrolls   []event.ScrollEvent
	pinches   []event.PinchEvent
	pinchEnds int
}

func newGestureTarget() *gestureTarget {
	t := &gestureTarget{Rectangle: *canvas.NewRectangle(color.White)}
	t.SetMinSize(geo.NewSize(200, 200))
	return t
}

func (t *gestureTarget) Scrolled(e *event.ScrollEvent) { t.scrolls = append(t.scrolls, *e) }
func (t *gestureTarget) Pinched(e *event.PinchEvent)   { t.pinches = append(t.pinches, *e) }
func (t *gestureTarget) PinchEnded()                   { t.pinchEnds++ }

type enderTarget struct {
	*gestureTarget
	ends int
}

func (t *enderTarget) ScrollEnded() { t.ends++ }

func phases(scrolls []event.ScrollEvent) []event.ScrollPhase {
	var p []event.ScrollPhase
	for _, s := range scrolls {
		p = append(p, s.Phase)
	}
	return p
}

// A touchpad gesture reaches a Scrollable with its phases, never as a scroll
// without movement: a start that does not move makes the first move the
// beginning. A ScrollEnder is told when the fingers lift, once, before the
// coast (#391).
func TestWindow_ScrollGesturePhases(t *testing.T) {
	target := &enderTarget{gestureTarget: newGestureTarget()}
	w := keyWindow(target)
	w.mousePos = geo.NewPos(50, 50)
	tp := event.ScrollSourceTouchpad

	w.processScrollGesture(0, 0, tp, gestureBegan, 0)
	assert.Empty(t, target.scrolls, "a start without movement is not a scroll")
	w.processScrollGesture(0, 2, tp, gestureChanged, 0)
	w.processScrollGesture(0, 1, tp, gestureChanged, 0)
	assert.Zero(t, target.ends)
	w.processScrollGesture(0, 0, tp, gestureEnded, 0)
	assert.Equal(t, 1, target.ends, "told the fingers lifted")
	w.processScrollGesture(0, 3, tp, gestureMomentum, 0)
	w.processScrollGesture(0, 0, tp, gestureEnded, 0)
	assert.Equal(t, 1, target.ends, "told once per gesture")

	assert.Equal(t, []event.ScrollPhase{event.ScrollPhaseBegan, event.ScrollPhaseChanged, event.ScrollPhaseMomentum},
		phases(target.scrolls))
	for _, s := range target.scrolls {
		assert.NotZero(t, s.Scrolled, "every scroll moves")
		assert.Equal(t, tp, s.Source)
	}

	// A start that moves is the beginning itself; a cancel ends it too.
	target.scrolls = nil
	w.processScrollGesture(0, 2, tp, gestureBegan, 0)
	w.processScrollGesture(0, 0, tp, gestureCancelled, 0)
	assert.Equal(t, []event.ScrollPhase{event.ScrollPhaseBegan}, phases(target.scrolls))
	assert.Equal(t, 2, target.ends)

	// A wheel has no phase and no end.
	target.scrolls = nil
	w.processScrollGesture(0, -1, event.ScrollSourceMouseWheel, gestureNone, 0)
	assert.Equal(t, []event.ScrollPhase{event.ScrollPhaseUnknown}, phases(target.scrolls))
	assert.Equal(t, 2, target.ends)
}

// A Scrollable that is not a ScrollEnder sees the same scrolls and nothing
// else: no scroll without movement.
func TestWindow_ScrollGestureLeavesPlainScrollablesAlone(t *testing.T) {
	target := newGestureTarget()
	w := keyWindow(target)
	w.mousePos = geo.NewPos(50, 50)
	tp := event.ScrollSourceTouchpad
	w.processScrollGesture(0, 0, tp, gestureBegan, 0)
	w.processScrollGesture(0, 2, tp, gestureChanged, 0)
	w.processScrollGesture(0, 0, tp, gestureEnded, 0)
	require.Len(t, target.scrolls, 1)
	assert.Equal(t, event.ScrollPhaseBegan, target.scrolls[0].Phase)
}

// A trackpad pinch goes to the Pinchable under the pointer when it began,
// each step at the pointer, and ends there (#391).
func TestWindow_TrackpadPinch(t *testing.T) {
	target := newGestureTarget()
	w := keyWindow(target)
	w.mousePos = geo.NewPos(50, 60)

	w.processTrackpadPinch(1.1, gestureBegan)
	w.processTrackpadPinch(0.9, gestureChanged)
	assert.Zero(t, target.pinchEnds)
	w.processTrackpadPinch(1, gestureEnded)
	assert.Equal(t, 1, target.pinchEnds)
	require.Len(t, target.pinches, 2)
	assert.InDelta(t, 1.1, target.pinches[0].Scale, 1e-6)
	assert.InDelta(t, 0.9, target.pinches[1].Scale, 1e-6)
	assert.Equal(t, geo.NewPos(50, 60), target.pinches[0].AbsolutePosition)
	origin := driver.AbsolutePositionForObject(target, w.canvas.ObjectTrees())
	assert.Equal(t, geo.NewPos(50, 60).Subtract(origin), target.pinches[0].Position, "relative to the target")

	// Over nothing that pinches, nothing happens.
	w.mousePos = geo.NewPos(5000, 5000)
	w.processTrackpadPinch(1.1, gestureBegan)
	w.processTrackpadPinch(1, gestureEnded)
	assert.Len(t, target.pinches, 2)
	assert.Equal(t, 1, target.pinchEnds)
}

// ctrl+wheel over a Pinchable pinches it instead of scrolling, a notch zooming
// by a tenth, and the pinch ends a moment after the last step (#391).
func TestWindow_CtrlWheelPinches(t *testing.T) {
	var ends []func()
	old := afterWheelPinchStep
	afterWheelPinchStep = func(f func()) { ends = append(ends, f) }
	t.Cleanup(func() { afterWheelPinchStep = old })

	target := newGestureTarget()
	w := keyWindow(target)
	w.mousePos = geo.NewPos(50, 50)
	ctrl := event.KeyModifierControl
	wheel := event.ScrollSourceMouseWheel

	w.processScrollGesture(0, 1, wheel, gestureNone, ctrl)
	w.processScrollGesture(0, -0.5, wheel, gestureNone, ctrl)
	assert.Empty(t, target.scrolls, "ctrl+wheel pinches rather than scrolls")
	require.Len(t, target.pinches, 2)
	assert.InDelta(t, 1.1, target.pinches[0].Scale, 1e-6, "a notch up zooms in by a tenth")
	assert.InDelta(t, math.Exp(-0.5*math.Log(1.1)), target.pinches[1].Scale, 1e-6)

	require.Len(t, ends, 2)
	ends[0]()
	assert.Zero(t, target.pinchEnds, "an end a later step overtook does nothing")
	ends[1]()
	assert.Equal(t, 1, target.pinchEnds, "the pinch ends after its last step")

	// A ctrl+wheel step that does not zoom (a sideways tilt) sends no pinch
	// step of 1, and does not scroll either.
	w.processScrollGesture(2, 0, wheel, gestureNone, ctrl)
	assert.Len(t, target.pinches, 2)
	assert.Empty(t, target.scrolls)

	// Without ctrl it scrolls.
	w.processScrollGesture(0, 1, wheel, gestureNone, 0)
	assert.Len(t, target.scrolls, 1)
}

// ctrl+wheel over something that does not pinch still scrolls it.
func TestWindow_CtrlWheelScrollsWhereNothingPinches(t *testing.T) {
	target := &scrollOnly{gestureTarget: newGestureTarget()}
	w := keyWindow(target)
	w.mousePos = geo.NewPos(50, 50)
	w.processScrollGesture(0, 1, event.ScrollSourceMouseWheel, gestureNone, event.KeyModifierControl)
	assert.Len(t, target.scrolls, 1)
}

// scrollOnly is a Scrollable that does not pinch.
type scrollOnly struct{ *gestureTarget }

func (s *scrollOnly) Pinched() {} // not event.Pinchable: wrong signature

// A browser pinch arrives as ctrl+wheel steps whose deltaY add up to
// -100 ln(scale) (measured in Chrome: a synthesized 2x pinch sent 18 events
// summing to -69.31). The web driver gets them negated, and their product
// comes back to the pinch's own scale. A mouse notch, one 100 px step, zooms
// by a tenth, as on the desktop (#391).
func TestBrowserWheelPinchScale(t *testing.T) {
	total := 100 * math.Log(2) // a 2x pinch, as the web driver sees it
	product := 1.0
	for i := 0; i < 18; i++ {
		product *= browserWheelPinchScale(total / 18)
	}
	assert.InDelta(t, 2, product, 1e-9, "the steps of a 2x pinch make 2x")

	product = 1
	for i := 0; i < 18; i++ {
		product *= browserWheelPinchScale(-total / 18)
	}
	assert.InDelta(t, 0.5, product, 1e-9)

	assert.InDelta(t, 1.1, browserWheelPinchScale(100), 1e-9, "a notch up zooms in by a tenth")
	assert.InDelta(t, 1/1.1, browserWheelPinchScale(-100), 1e-9, "a notch down zooms out by a tenth")
	assert.Equal(t, 1.0, browserWheelPinchScale(0))
}
