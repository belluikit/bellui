package dagwidget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// The mouse on a palette entry is unchanged by the touch long-press
// (bellui#1296). Each case replays the calls the desktop driver makes, in its
// order: MouseDown on press; LongPressed if the button is held still for the
// long-press delay; Dragged once the pointer moves; on release MouseUp, then
// DragEnd after a drag, or Tapped after a click that did not hold-fire. The
// driver never asks ClaimsDrag for a mouse drag.
func TestPaletteItem_MouseClickAndDragUnchanged(t *testing.T) {
	primary := &desktop.MouseEvent{Button: desktop.MouseButtonPrimary}
	drop := &event.DragEvent{PointEvent: event.PointEvent{AbsolutePosition: geo.NewPos(400, 300)}}

	for _, tc := range []struct {
		name            string
		hold, drag, tap bool
	}{
		{name: "a quick click adds a node", tap: true},
		{name: "a slow click adds a node", hold: true},
		{name: "a drag drops a node at once", drag: true},
		{name: "a hold then a drag drops one node", hold: true, drag: true},
	} {
		c := newTestCanvas(t)
		item := testPaletteItem(t, c)

		item.MouseDown(primary)
		if tc.hold {
			item.LongPressed(&event.PointEvent{})
		}
		if tc.drag {
			item.Dragged(drop)
		}
		item.MouseUp(primary)
		if tc.drag {
			item.DragEnd()
		}
		if tc.tap {
			item.Tapped(&event.PointEvent{})
		}
		assert.Len(t, c.Graph.Nodes, 1, tc.name)
	}
}

// A right click held still adds nothing: it is not a press-and-hold.
func TestPaletteItem_SecondaryMouseHoldAddsNothing(t *testing.T) {
	c := newTestCanvas(t)
	item := testPaletteItem(t, c)
	secondary := &desktop.MouseEvent{Button: desktop.MouseButtonSecondary}
	item.MouseDown(secondary)
	item.MouseUp(secondary)
	assert.Empty(t, c.Graph.Nodes)
}
