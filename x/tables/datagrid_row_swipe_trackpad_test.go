//go:build (ci || android || ios || mobile) && !wasm

// Two-finger trackpad swipes (#1483), delivered as the desktop driver
// delivers a touchpad gesture's scrolls (internal/driver/glfw,
// processScrollGesture): each scroll is offered along the chain of
// scrollables under the pointer, innermost first, until one takes it, and the
// chain's ScrollEnders are told when the fingers lift. The fixture is the
// touch tests' swipeGrid.

package tables

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
)

// padChain returns the scrollables under pos, innermost first, as the driver
// finds them.
func (sg *swipeGrid) padChain(pos geo.Position) []intdriver.ObjectAtPosition {
	return intdriver.FindAllObjectsAtPositionMatching(pos, func(o core.CanvasObject) bool {
		_, ok := o.(event.Scrollable)
		return ok
	}, nil, sg.w.Canvas().Content())
}

// padScroll offers one scroll along the chain under pos, as the driver's
// dispatchScrollChainPhase does.
func (sg *swipeGrid) padScroll(pos geo.Position, d geo.Delta, source event.ScrollSource, phase event.ScrollPhase) {
	ev := &event.ScrollEvent{Scrolled: d, Source: source, Phase: phase}
	ev.AbsolutePosition = pos
	for _, oap := range sg.padChain(pos) {
		ev.Position = oap.Position
		if c, ok := oap.Object.(interface {
			TryScrolled(*event.ScrollEvent) bool
		}); ok {
			if c.TryScrolled(ev) {
				return
			}
			continue
		}
		oap.Object.(event.Scrollable).Scrolled(ev)
		return
	}
}

// padGesture is a two-finger trackpad gesture over pos: each step a scroll
// while the fingers move, the first one marked as the beginning, then the
// fingers lift, and the coast's scrolls follow.
func (sg *swipeGrid) padGesture(pos geo.Position, steps []geo.Delta, coast ...geo.Delta) {
	var enders []event.ScrollEnder
	for _, oap := range sg.padChain(pos) {
		if e, ok := oap.Object.(event.ScrollEnder); ok {
			enders = append(enders, e)
		}
	}
	for i, d := range steps {
		phase := event.ScrollPhaseChanged
		if i == 0 {
			phase = event.ScrollPhaseBegan
		}
		sg.padScroll(pos, d, event.ScrollSourceTouchpad, phase)
	}
	for _, e := range enders {
		e.ScrollEnded()
	}
	for _, d := range coast {
		sg.padScroll(pos, d, event.ScrollSourceTouchpad, event.ScrollPhaseMomentum)
	}
}

// padSwipe is a gesture of n equal sideways steps adding up to dx.
func (sg *swipeGrid) padSwipe(pos geo.Position, dx float32, n int) {
	steps := make([]geo.Delta, n)
	for i := range steps {
		steps[i] = geo.NewDelta(dx/float32(n), 0)
	}
	sg.padGesture(pos, steps)
}

// The grid is told when a touchpad gesture's fingers lift, and its rows are
// offered each scroll before the body's Scroll is.
func TestRowSwipe_TrackpadChain(t *testing.T) {
	sg := newSwipeGrid(t)
	chain := sg.padChain(sg.rowCenter(3, 100))
	require.NotEmpty(t, chain)
	_, first := chain[0].Object.(interface {
		TryScrolled(*event.ScrollEvent) bool
	})
	assert.True(t, first, "the innermost scrollable over a row offers the grid each scroll first")
	assert.NotSame(t, sg.g.scrollContent.Scroll, chain[0].Object, "and it is not the body's Scroll")
	var ender bool
	for _, oap := range chain {
		_, ok := oap.Object.(event.ScrollEnder)
		ender = ender || ok
	}
	assert.True(t, ender, "the grid is a ScrollEnder in the chain, so it is told when the fingers lift")
}

// A two-finger swipe left over a row opens its trailing actions, and one to
// the right its leading ones, without scrolling the body.
func TestRowSwipe_TrackpadOpensTheRow(t *testing.T) {
	sg := newSwipeGrid(t)
	g := sg.g
	sg.padSwipe(sg.rowCenter(3, 100), -100, 10)
	require.Equal(t, 3, g.swipe.src, "row 3 is open")
	assert.Equal(t, -sg.deleteWidth(), g.swipe.offset, "it rests open on its trailing actions")
	assert.Equal(t, geo.Position{}, g.scrollContent.Offset(), "the body did not scroll")

	g.closeSwipe(false)
	sg.padSwipe(sg.rowCenter(5, 100), 100, 10)
	require.Equal(t, 5, g.swipe.src)
	assert.Equal(t, g.swipeActionsWidth(g.LeadingSwipeActions(0)), g.swipe.offset, "a swipe right opens the leading actions")
}

// The edge rule holds for a trackpad as for touch: the gesture scrolls the
// body while it can, one that scrolled into the edge stays a scroll, and one
// that starts at the edge swipes.
func TestRowSwipe_TrackpadEdgeRule(t *testing.T) {
	sg := newSwipeGrid(t, 200, 200, 200)
	g := sg.g
	pos := sg.rowCenter(3, 150)

	sg.padSwipe(pos, -60, 6)
	assert.Equal(t, float32(60), g.scrollContent.Offset().X, "away from the edge, the gesture scrolls")
	assert.Equal(t, -1, g.swipe.src)

	maxX := g.cellsWidget.MinSize().Width - g.scrollContent.Scroll.Size().Width
	require.Greater(t, maxX, float32(60))
	sg.padSwipe(pos, -(maxX - 60 + 150), 20)
	assert.Equal(t, maxX, g.scrollContent.Offset().X, "the gesture scrolled to the right edge")
	assert.Equal(t, -1, g.swipe.src, "a gesture that scrolled into the edge did not swipe")

	sg.padSwipe(pos, -100, 10)
	assert.Equal(t, 3, g.swipe.src, "a gesture that starts at the edge swipes the row")
	assert.Equal(t, maxX, g.scrollContent.Offset().X, "and does not scroll")
}

// Once a row is being swiped, the gesture keeps it: fingers drifting up or
// down a little do not scroll the body, and the row follows them sideways.
func TestRowSwipe_TrackpadSwipeKeepsTheGesture(t *testing.T) {
	sg := newSwipeGrid(t)
	g := sg.g
	steps := []geo.Delta{geo.NewDelta(-10, 0)}
	for i := 0; i < 9; i++ {
		steps = append(steps, geo.NewDelta(-10, 4))
	}
	sg.padGesture(sg.rowCenter(3, 100), steps)
	assert.Equal(t, 3, g.swipe.src)
	assert.Zero(t, g.scrollContent.Offset().Y, "the drift did not scroll the body")
}

// The coast after a swipe moves nothing: neither the row nor the body. The
// coast after a scroll still scrolls, and a coast never starts a swipe.
func TestRowSwipe_TrackpadCoast(t *testing.T) {
	sg := newSwipeGrid(t)
	g := sg.g
	pos := sg.rowCenter(3, 100)

	sg.padGesture(pos, []geo.Delta{geo.NewDelta(-50, 0), geo.NewDelta(-50, 0)},
		geo.NewDelta(-30, 0), geo.NewDelta(0, -30), geo.NewDelta(-10, -10))
	require.Equal(t, 3, g.swipe.src)
	assert.Equal(t, -sg.deleteWidth(), g.swipe.offset, "the coast did not move the open row")
	assert.Equal(t, geo.Position{}, g.scrollContent.Offset(), "nor scroll the body")

	g.closeSwipe(false)
	sg.padGesture(pos, []geo.Delta{geo.NewDelta(0, -20)}, geo.NewDelta(0, -15))
	assert.Equal(t, float32(35), g.scrollContent.Offset().Y, "a scroll's coast scrolls on")

	// A coast that arrives on its own, sideways at the edge, swipes nothing.
	sg.padScroll(pos, geo.NewDelta(-40, 0), event.ScrollSourceTouchpad, event.ScrollPhaseMomentum)
	assert.Equal(t, -1, g.swipe.src, "a coast never starts a swipe")
}

// Scrolls with no gesture never swipe: a mouse wheel, and a touchpad on a
// platform that reports no phases (Windows, X11, the browsers).
func TestRowSwipe_TrackpadNeedsAGesture(t *testing.T) {
	sg := newSwipeGrid(t)
	pos := sg.rowCenter(3, 100)
	for _, src := range []event.ScrollSource{event.ScrollSourceMouseWheel, event.ScrollSourceTouchpad, event.ScrollSourceUnknown} {
		for i := 0; i < 10; i++ {
			sg.padScroll(pos, geo.NewDelta(-10, 0), src, event.ScrollPhaseUnknown)
		}
		assert.Equal(t, -1, sg.g.swipe.src, "source %v with no phase does not swipe", src)
	}
	// The wheel still scrolls the body down the chain.
	sg.padScroll(pos, geo.NewDelta(0, -25), event.ScrollSourceMouseWheel, event.ScrollPhaseUnknown)
	assert.Equal(t, float32(25), sg.g.scrollContent.Offset().Y, "a wheel scroll reaches the body")
}

// With a row open, a gesture over another row closes it and scrolls; a
// sideways one on the open row moves it.
func TestRowSwipe_TrackpadWithARowOpen(t *testing.T) {
	sg := newSwipeGrid(t)
	g := sg.g
	sg.padSwipe(sg.rowCenter(3, 100), -100, 10)
	require.Equal(t, 3, g.swipe.src)

	sg.padSwipe(sg.rowCenter(3, 100), 100, 10)
	assert.Equal(t, -1, g.swipe.src, "swiped back on the open row, it closes")

	sg.padSwipe(sg.rowCenter(3, 100), -100, 10)
	require.Equal(t, 3, g.swipe.src)
	sg.padGesture(sg.rowCenter(6, 100), []geo.Delta{geo.NewDelta(0, -30)})
	assert.Equal(t, -1, g.swipe.src, "a gesture over another row closes the open one")
	assert.Equal(t, float32(30), g.scrollContent.Offset().Y, "and scrolls")
}

// Right to left, the leading actions are on the right, and a right-to-left
// grid that scrolls sideways starts at its leading edge, where a gesture
// toward the trailing edge scrolls and one toward the leading edge swipes.
func TestRowSwipe_TrackpadRightToLeft(t *testing.T) {
	bellui.SetLayoutDirection(bellui.DirectionRTL)
	defer bellui.SetLayoutDirection(bellui.DirectionAuto)

	sg := newSwipeGrid(t)
	g := sg.g
	sg.padSwipe(sg.rowCenter(3, 200), -100, 10)
	require.Equal(t, 3, g.swipe.src)
	assert.Equal(t, -g.swipeActionsWidth(g.LeadingSwipeActions(0)), g.swipe.offset,
		"a swipe left opens the leading actions, on the right")

	wide := newSwipeGrid(t, 200, 200, 200)
	g = wide.g
	start := g.scrollContent.Offset().X
	require.Greater(t, start, float32(0), "it opens at its leading edge, the right")
	pos := wide.rowCenter(3, 150)
	wide.padSwipe(pos, 100, 10)
	assert.Equal(t, 3, g.swipe.src, "at the leading edge, a gesture toward it swipes")
	assert.Equal(t, start, g.scrollContent.Offset().X, "and does not scroll")

	g.closeSwipe(false)
	wide.padSwipe(pos, -60, 6)
	assert.Equal(t, -1, g.swipe.src, "a gesture toward the trailing edge scrolls")
	assert.NotEqual(t, start, g.scrollContent.Offset().X)
}

// A gesture over a pinned column swipes the whole row too.
func TestRowSwipe_TrackpadOverAPinnedColumn(t *testing.T) {
	sg := newSwipeGrid(t, 100, 200, 200)
	g := sg.g
	g.API().SetColumnPinned("n", PinLeft)
	g.Refresh()
	maxX := g.cellsWidget.MinSize().Width - g.scrollContent.Scroll.Size().Width
	require.Greater(t, maxX, float32(0))
	g.scrollContent.ScrollTo(geo.NewPos(maxX, 0))

	sg.padSwipe(sg.rowCenter(4, 50), -100, 10)
	assert.Equal(t, 4, g.swipe.src, "a gesture over the pinned column swipes the row")
}

// A disabled grid swipes nothing.
func TestRowSwipe_TrackpadDisabled(t *testing.T) {
	sg := newSwipeGrid(t)
	sg.g.Disable()
	sg.padSwipe(sg.rowCenter(3, 100), -100, 10)
	assert.Equal(t, -1, sg.g.swipe.src)
}

// A scroll offered only to the innermost scrollable, as the accessibility
// walker's scroll target is, still scrolls the grid's body (#82).
func TestRowSwipe_CellsForwardAScroll(t *testing.T) {
	sg := newSwipeGrid(t)
	sg.g.cellsWidget.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -40)})
	assert.Equal(t, float32(40), sg.g.scrollContent.Offset().Y)
}

// A gesture toward an edge the row has no actions on is not a swipe: it stays
// a scroll, so the fingers' drift scrolls the body.
func TestRowSwipe_TrackpadSideWithoutActions(t *testing.T) {
	sg := newSwipeGrid(t)
	sg.g.LeadingSwipeActions = nil
	steps := []geo.Delta{geo.NewDelta(10, 0)}
	for i := 0; i < 5; i++ {
		steps = append(steps, geo.NewDelta(10, -6))
	}
	sg.padGesture(sg.rowCenter(3, 100), steps)
	assert.Equal(t, -1, sg.g.swipe.src, "no leading actions, nothing to reveal on the left")
	assert.Equal(t, float32(30), sg.g.scrollContent.Offset().Y, "the gesture stayed a scroll")
}

// A gesture's first scroll decides: one that starts up or down is not a
// swipe, even where the body cannot scroll that way any more, and stays none
// when the fingers then turn sideways.
func TestRowSwipe_TrackpadVerticalAtTheEnd(t *testing.T) {
	sg := newSwipeGrid(t)
	g := sg.g
	bottom := g.cellsWidget.MinSize().Height - g.scrollContent.Scroll.Size().Height
	require.Greater(t, bottom, float32(0))
	g.scrollContent.ScrollTo(geo.NewPos(0, bottom))
	steps := []geo.Delta{geo.NewDelta(0, -20)}
	for i := 0; i < 10; i++ {
		steps = append(steps, geo.NewDelta(-10, 0))
	}
	sg.padGesture(sg.rowCenter(g.RowCount()-2, 100), steps)
	assert.Equal(t, -1, g.swipe.src)
}
