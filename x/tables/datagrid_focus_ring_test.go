package tables

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/test"
)

// ringIn returns the focus ring drawn in a band, searching inside its
// containers and clips: a stroked rectangle with no fill, or nil.
func ringIn(t *testing.T, band core.Widget) *canvas.Rectangle {
	t.Helper()
	var walk func(objs []core.CanvasObject) *canvas.Rectangle
	walk = func(objs []core.CanvasObject) *canvas.Rectangle {
		for _, o := range objs {
			if !o.Visible() {
				continue
			}
			switch v := o.(type) {
			case *canvas.Rectangle:
				if v.StrokeWidth == 2 {
					return v
				}
			case *bellui.Container:
				if r := walk(v.Objects); r != nil {
					return r
				}
			case core.Widget:
				if r := walk(test.TempWidgetRenderer(t, v).Objects()); r != nil {
					return r
				}
			}
		}
		return nil
	}
	return walk(test.TempWidgetRenderer(t, band).Objects())
}

// While the grid has keyboard focus, the cursor's cell is outlined, in the
// main band or a pinned one; without focus nothing is (#1512). The cursor was
// only announced to screen readers before, so Tab, the arrows and
// Option+Up/Down showed nothing on screen.
func TestDataGrid_KeyboardCursorIsDrawn(t *testing.T) {
	g, _ := pressGrid(t) // columns a and b, 100 wide each
	g.currentFocus = CellID{Row: 2, Col: 1}
	g.Refresh()
	assert.Nil(t, ringIn(t, g.cellsWidget), "an unfocused grid draws no cursor")

	g.FocusGained()
	ring := ringIn(t, g.cellsWidget)
	require.NotNil(t, ring, "a focused grid draws its cursor")
	x, w := g.viewport.ColX(1)
	y, h := g.viewport.RowY(2)
	assert.Equal(t, x, ring.Position().X)
	assert.Equal(t, y, ring.Position().Y)
	assert.Equal(t, w, ring.Size().Width)
	assert.Equal(t, h, ring.Size().Height)

	// The cursor moves with the keys.
	g.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	ring = ringIn(t, g.cellsWidget)
	require.NotNil(t, ring)
	y3, _ := g.viewport.RowY(3)
	assert.Equal(t, y3, ring.Position().Y)

	g.FocusLost()
	assert.Nil(t, ringIn(t, g.cellsWidget), "losing focus hides the cursor")
}

// A cursor in a pinned column is drawn by that band.
func TestDataGrid_KeyboardCursorInAPinnedBand(t *testing.T) {
	g, _ := pressGrid(t)
	g.API().SetColumnPinned("a", PinLeft)
	g.currentFocus = CellID{Row: 1, Col: 0}
	g.FocusGained()
	assert.NotNil(t, ringIn(t, g.pinnedLeftCells), "the pinned band draws the cursor")
	assert.Nil(t, ringIn(t, g.cellsWidget), "the main band does not")
}
