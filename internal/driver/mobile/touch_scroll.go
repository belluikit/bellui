//go:build !wasm

package mobile

import (
	"math"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/driver/common"
)

// Two-finger pan scrolling (#402). The shared touch handler turns two fingers
// moving together into scroll steps through its ScrollCallback (#398); the
// desktop touchscreen and the browser set one, and this is the mobile driver's.
// One finger still scrolls by dragging, because the mobile Scroll is Draggable
// (internal/widget/scroller_mobile.go): a drag latches the Scroll before the
// handler's own one-finger scroll path is ever tried.
//
// A pan reaches the scrollables under the point where the second finger
// landed, innermost first, each passing a step on when it is at its boundary or
// declines touch scrolls (a chart keeps one finger and leaves two to the page).
// As in the glfw driver, that chain is taken at the gesture's first step and
// kept for the rest of it and its momentum, so a widget sliding under the
// fingers as the page scrolls cannot take over the gesture (#416).

// touchScrollLatch is the chain of scrollables one gesture scrolls.
type touchScrollLatch struct {
	gen   int // the touchScrollGen it was taken in; a new contact starts a new gesture
	ok    bool
	chain []intdriver.ObjectAtPosition
}

// initTouchScroll opts the canvas into touch-sourced scrolling.
func (c *canvas) initTouchScroll() {
	c.ScrollCallback = func(origin geo.Position, delta geo.Delta) {
		c.dispatchTouchScroll(origin, delta)
	}
	c.ScrollEndCallback = c.startTouchScrollMomentum
}

// TapDown starts a new touch gesture, which stops any momentum still running
// from the last one (touch-to-stop), and then hands the contact to the shared
// handler.
func (c *canvas) TapDown(pos geo.Position, tapID int) {
	c.touchScrollGen++
	c.TouchHandler.TapDown(pos, tapID)
}

// dispatchTouchScroll delivers one step of a touch scroll to the gesture's
// latched chain, taking the chain on the gesture's first step. It reports
// whether a scrollable consumed the step.
func (c *canvas) dispatchTouchScroll(origin geo.Position, delta geo.Delta) bool {
	l := &c.touchScroll
	if !l.ok || l.gen != c.touchScrollGen {
		*l = touchScrollLatch{gen: c.touchScrollGen, ok: true, chain: c.scrollablesAt(origin)}
	}
	return c.dispatchScrollChain(l.chain, origin, delta)
}

// scrollablesAt returns every scrollable under pos, innermost first, from the
// top overlay if it has one there and from the content otherwise, the way
// FindObjectAtPositionMatching resolves taps.
func (c *canvas) scrollablesAt(pos geo.Position) []intdriver.ObjectAtPosition {
	isScrollable := func(object core.CanvasObject) bool {
		_, ok := object.(event.Scrollable)
		return ok
	}
	if overlay := c.Overlays().Top(); overlay != nil {
		if found := intdriver.FindAllObjectsAtPositionMatching(pos, isScrollable, overlay); len(found) > 0 {
			return found
		}
	}
	if c.menu != nil {
		return intdriver.FindAllObjectsAtPositionMatching(pos, isScrollable, nil, c.menu)
	}
	return intdriver.FindAllObjectsAtPositionMatching(pos, isScrollable, nil, c.windowHead, c.content)
}

// dispatchScrollChain offers a touch scroll at pos to the scrollables of
// chain, innermost first, until one consumes it. Each one's relative position
// is worked out again from where it is now, because the chain is kept for a
// whole gesture and moves while the gesture scrolls it.
func (c *canvas) dispatchScrollChain(chain []intdriver.ObjectAtPosition, pos geo.Position, delta geo.Delta) bool {
	ev := &event.ScrollEvent{}
	ev.Scrolled = delta
	ev.AbsolutePosition = pos
	ev.Source = event.ScrollSourceTouch
	for _, oap := range chain {
		ev.Position = pos.Subtract(intdriver.AbsolutePositionForObject(oap.Object, c.ObjectTrees()))
		if consumer, ok := oap.Object.(intdriver.ScrollConsumer); ok {
			if consumer.TryScrolled(ev) {
				return true
			}
			continue
		}
		// A plain Scrollable has no way to pass a step on, so it takes it.
		if wid, ok := oap.Object.(event.Scrollable); ok {
			wid.Scrolled(ev)
			return true
		}
	}
	return false
}

// startTouchScrollMomentum carries a released pan on with the same decay curve
// as a one-finger flick (common.DragDecay, about 60 steps a second), through
// the gesture's chain. It stops when the step decays out, when no scrollable
// consumes it any more (content flung against its end must not look stuck
// there), or when a new contact lands.
func (c *canvas) startTouchScrollMomentum(origin geo.Position, delta geo.Delta) {
	if math.Abs(float64(delta.DX)) <= common.TapMoveEndThreshold && math.Abs(float64(delta.DY)) <= common.TapMoveEndThreshold {
		return // a controlled pan's release, not a flick
	}
	gen := c.touchScrollGen
	chain := c.touchScroll.chain
	go func() {
		for {
			time.Sleep(time.Millisecond * 16)
			delta = common.DragDecay(delta)
			if delta == (geo.Delta{}) {
				return
			}
			stop := false
			bellui.DoAndWait(func() {
				if c.touchScrollGen != gen || !c.dispatchScrollChain(chain, origin, delta) {
					stop = true
				}
			})
			if stop {
				return
			}
		}
	}()
}
