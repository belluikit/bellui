package tables

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// The a11y walker keys a virtual cell node's scroll target on its provider
// widget implementing event.Scrollable (pre-rebrand tracker #953) — losing this assertion
// makes assistive-tech scroll gestures inert over the grid again (#82).
var _ event.Scrollable = (*DataGrid)(nil)

// An AT scroll gesture lands on the grid widget itself (not the internal
// scroll the walker never sees) and pages the body with wheel semantics:
// clamped at the edges, grid offset/viewport kept in sync.
func TestDataGrid_Scrolled_PagesTheGrid(t *testing.T) {
	g := newTestGrid(t, manyRows(1000), []*ColumnDef{{ColID: "n", Field: "n"}})
	layOut(t, g)

	require.Equal(t, geo.NewPos(0, 0), g.scrollContent.Offset())

	// Gesture down (wheel semantics: negative DY moves the content down).
	g.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -100)})
	afterDown := g.scrollContent.Offset()
	assert.Greater(t, afterDown.Y, float32(0), "gesture scrolled the body down")
	assert.Equal(t, afterDown, g.offset,
		"grid's cached offset synced via onScrolled, like a wheel event")

	// Gesture back up past the start clamps at the top.
	g.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, 1e6)})
	assert.Equal(t, float32(0), g.scrollContent.Offset().Y, "clamped at the top")

	// Gesture far past the end clamps at the bottom and stays there.
	g.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -1e9)})
	bottom := g.scrollContent.Offset().Y
	assert.Greater(t, bottom, afterDown.Y, "landed past the first page")
	g.Scrolled(&event.ScrollEvent{Scrolled: geo.NewDelta(0, -100)})
	assert.Equal(t, bottom, g.scrollContent.Offset().Y, "clamped at the bottom")
}

// A finger dragging the body scrolls the grid. The grid is the Draggable the
// touch handler latches, so before this a touch drag reached Dragged with no
// mouse gesture in progress and did nothing: the grid could not be scrolled
// on a phone. A mouse drag over the body still does not scroll it.
func TestDataGrid_TouchDragScrollsTheBody(t *testing.T) {
	g := newTestGrid(t, manyRows(1000), []*ColumnDef{{ColID: "n", Field: "n"}})
	layOut(t, g)
	body := geo.NewPos(40, g.HeaderHeight()+40)
	drag := &event.DragEvent{PointEvent: event.PointEvent{Position: body}, Dragged: geo.NewDelta(0, -120)}

	g.Dragged(drag) // touch: no MouseDown came first
	g.DragEnd()
	assert.Greater(t, g.scrollContent.Offset().Y, float32(0), "a finger moving up scrolls the body down")

	scrolled := g.scrollContent.Offset()
	g.MouseDown(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: body}, Button: desktop.MouseButtonPrimary})
	g.Dragged(drag)
	g.DragEnd()
	g.MouseUp(&desktop.MouseEvent{PointEvent: event.PointEvent{Position: body}, Button: desktop.MouseButtonPrimary})
	assert.Equal(t, scrolled, g.scrollContent.Offset(), "a mouse drag must not scroll the body")
}
