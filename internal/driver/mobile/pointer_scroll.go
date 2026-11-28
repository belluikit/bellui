//go:build !wasm

package mobile

import (
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/driver/mobile/event/scroll"
)

// Pointer scrolling on a touch device (#1510): a mouse wheel, or a touchpad's
// two-finger scroll, on Android or an iPad. It reaches the scrollables under
// the pointer as the glfw driver's wheel and trackpad scrolls do, with the
// same sources and phases: a wheel step is ScrollSourceMouseWheel with no
// phase, and a touchpad gesture is ScrollSourceTouchpad, Began then Changed,
// with its ScrollEnders told when the fingers lift. Neither platform reports
// a coast after the fingers lift, so there is no momentum phase.
//
// As with a two-finger touch pan (touch_scroll.go, #416), a touchpad gesture
// keeps the chain of scrollables it began over, so content sliding under the
// pointer as it scrolls cannot take the gesture over.

// pointerScrollGesture is the touchpad gesture under way on a canvas.
type pointerScrollGesture struct {
	active bool
	// began is set until the gesture's first step that moves, which is
	// reported as its beginning: a Scrollable never gets a scroll without
	// movement.
	began bool
	chain []intdriver.ObjectAtPosition
	pos   geo.Position
}

// PointerScrolled delivers a pointer scroll at pos, in canvas coordinates,
// by delta.
func (c *canvas) PointerScrolled(pos geo.Position, delta geo.Delta, touchpad bool, phase scroll.Phase) {
	if !touchpad {
		if delta != (geo.Delta{}) {
			c.dispatchScrollChainAs(c.scrollablesAt(pos), pos, delta, event.ScrollSourceMouseWheel, event.ScrollPhaseUnknown)
		}
		return
	}

	g := &c.pointerScroll
	switch phase {
	case scroll.PhaseBegan:
		c.endPointerScroll() // an end the platform never sent
		*g = pointerScrollGesture{active: true, began: true, chain: c.scrollablesAt(pos), pos: pos}
		return
	case scroll.PhaseEnded:
		c.endPointerScroll()
		return
	}
	if !g.active {
		// A step with no beginning: the gesture began before this canvas
		// saw it. It starts here.
		*g = pointerScrollGesture{active: true, began: true, chain: c.scrollablesAt(pos), pos: pos}
	}
	if delta == (geo.Delta{}) {
		return
	}
	p := event.ScrollPhaseChanged
	if g.began {
		p = event.ScrollPhaseBegan
		g.began = false
	}
	c.dispatchScrollChainAs(g.chain, g.pos, delta, event.ScrollSourceTouchpad, p)
}

// endPointerScroll tells the ScrollEnders of the gesture under way that the
// fingers lifted.
func (c *canvas) endPointerScroll() {
	g := c.pointerScroll
	c.pointerScroll = pointerScrollGesture{}
	if !g.active {
		return
	}
	for _, oap := range g.chain {
		if e, ok := oap.Object.(event.ScrollEnder); ok {
			e.ScrollEnded()
		}
	}
}
