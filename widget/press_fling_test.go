package widget_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/internal/driver/common"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
)

// windowFinder hit-tests a window's content the way the mobile driver does.
type windowFinder struct{ c core.Canvas }

func (f windowFinder) FindObjectAtPositionMatching(pos geo.Position, match func(core.CanvasObject) bool) (core.CanvasObject, geo.Position, int) {
	return intdriver.FindObjectAtPositionMatching(pos, match, nil, f.c.Content())
}

func (f windowFinder) CanvasForTouch() core.Canvas { return f.c }

// TestList_FlingLeavesNoRowPressed is #1091's done-when: a touch that goes
// down on a row and flings the list cancels the row's press, so no row shows
// one while the list moves or after it. It drives the drivers' shared touch
// handler over a real list.
func TestList_FlingLeavesNoRowPressed(t *testing.T) {
	test.NewTempApp(t)
	list := widget.NewList(func() int { return 100 },
		func() core.CanvasObject { return widget.NewLabel("row") },
		func(widget.ListItemID, core.CanvasObject) {})
	list.OnSelected = func(widget.ListItemID) { t.Error("the fling selected a row") }
	w := test.NewTempWindow(t, list)
	w.Resize(geo.NewSize(200, 300))
	c := w.Canvas()

	h := &common.TouchHandler{}
	h.InitTouchHandler(windowFinder{c})
	// As the glfw driver does: the scroll goes to the Scrollable at its origin.
	h.ScrollCallback = func(origin geo.Position, delta geo.Delta) {
		o, pos, _ := intdriver.FindObjectAtPositionMatching(origin, func(o core.CanvasObject) bool {
			_, ok := o.(event.Scrollable)
			return ok
		}, nil, c.Content())
		require.NotNil(t, o, "no Scrollable under the touch")
		o.(event.Scrollable).Scrolled(&event.ScrollEvent{PointEvent: event.PointEvent{Position: pos}, Scrolled: delta})
	}
	drag := func(d event.Draggable, ev *event.DragEvent) { d.Dragged(ev) }

	h.TapDown(geo.NewPos(100, 150), 1)
	require.Equal(t, 1, widget.PressedListRowsForTest(c), "precondition: the touch pressed the row under it")
	for y := float32(130); y >= 30; y -= 20 {
		h.TapMove(geo.NewPos(100, y), 1, drag)
		assert.Zero(t, widget.PressedListRowsForTest(c), "a row stayed pressed while the list moved (finger at y=%v)", y)
	}
	h.TapUp(geo.NewPos(100, 30), 1,
		func(event.Tappable, *event.PointEvent) { t.Error("the fling's release tapped a row") },
		func(event.SecondaryTappable, *event.PointEvent) {},
		func(event.LongPressable, *event.PointEvent) {},
		func(event.DoubleTappable, *event.PointEvent) {},
		drag)
	assert.Zero(t, widget.PressedListRowsForTest(c), "a row was left pressed after the fling")
	assert.Positive(t, list.GetScrollOffset(), "precondition: the moves scrolled the list")
}
