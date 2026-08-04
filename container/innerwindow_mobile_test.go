//go:build mobile

package container

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// #263: the resize corner spans the touch-target floor on mobile while the
// visual indicator keeps its desktop size, and the layout keeps that target
// clear of the title-bar buttons when a window is shrunk to minimal content.
// The concrete floor value (icon-button size + padding ring) is asserted
// independently in the mobile driver's dispatch test; here the corner must
// agree with frameButtonTouchTarget, the same metric the frame buttons use.
func TestInnerWindow_CornerTouchTargetMobile(t *testing.T) {
	test.NewTempApp(t)

	w := NewInnerWindow("Title", widget.NewLabel("x"))
	target := frameButtonTouchTarget(w.Theme())

	corner := newDraggableCorner(w)
	assert.Equal(t, geo.NewSquareSize(target), corner.MinSize(),
		"the corner drag target must meet the touch floor on mobile")

	// The enlarged target's cost is drag-only (docs/notes/widgets.md): the
	// corner captures drags that start inside it, but must stay transparent
	// to every tap surface so taps in the square keep reaching the content
	// beneath. Growing the corner one of these interfaces would silently
	// widen the trade.
	_, tappable := any(corner).(event.Tappable)
	assert.False(t, tappable, "the corner must not capture taps")
	_, secondary := any(corner).(event.SecondaryTappable)
	assert.False(t, secondary, "the corner must not capture secondary taps")
	_, longPressable := any(corner).(event.LongPressable)
	assert.False(t, longPressable, "the corner must not capture long-presses")
	_, doubleTappable := any(corner).(event.DoubleTappable)
	assert.False(t, doubleTappable, "the corner must not capture double-taps")

	// Roomy window: the corner spans the full target.
	r := test.TempWidgetRenderer(t, w).(*innerWindowRenderer)
	w.Resize(geo.NewSize(300, 300))
	assert.Equal(t, geo.NewSquareSize(target), r.corner.Size())

	// Window shrunk to minimal content: the target is capped below the title
	// bar rather than stacked over the frame buttons.
	w.Resize(w.MinSize())
	assert.GreaterOrEqual(t, r.corner.Position().Y, w.barHeight(),
		"the corner must stay clear of the title bar at minimum size")

	// Window forced under its minimum: the cap can shrink the corner below
	// the indicator itself, and the pinning layout must clamp the indicator
	// inside the widget instead of pushing it out of the top-left.
	w.Resize(geo.NewSize(200, w.barHeight()+8))
	rendered := r.corner.(*draggableCorner)
	assert.Equal(t, float32(8), rendered.Size().Height,
		"the corner must be capped to the space below the title bar")
	indicator := test.TempWidgetRenderer(t, rendered).Objects()[0].(*bellui.Container).Objects[0]
	assert.LessOrEqual(t, indicator.Size().Height, rendered.Size().Height,
		"the indicator must shrink to fit a capped corner")
	assert.GreaterOrEqual(t, indicator.Position().Y, float32(0),
		"the indicator must stay inside the corner widget")
}
