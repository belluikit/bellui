package tables

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/theme"
)

var _ core.Disableable = (*DataGrid)(nil)

// DrawsDisabledLook reports that the paint walk must not dim a disabled
// DataGrid (bellui's core.DisabledLook, belluikit/bellui#1223): it draws its own text in the disabled colour; the content an app
// supplies (CellRenderer cells, master-detail content) is dimmed by the walk
// (belluikit/bellui#1393).
func (*DataGrid) DrawsDisabledLook() bool { return true }

// Disable makes the grid visible but inert (belluikit/player#641): it keeps
// its place and its rows, draws its own text in the disabled colour, and
// takes no input. Taps, double-taps and secondary taps select, sort, open,
// edit and fire nothing; drags neither resize, reorder nor scroll; the wheel
// and the scroll bars do not scroll; keys and shortcuts do nothing; the
// accessible actions of its cells, headers and disclosure rows are refused.
// Tab skips it and the parts it builds (filter entries, select checkboxes),
// and a focus already on one of them is dropped. An open inline editor is
// closed without committing, and an editor or a filter popup cannot be
// opened, by the user or by StartEditing and ShowFilterPopup.
//
// Content the app supplies (a column's CellRenderer cells, master-detail
// content) is the app's: the grid does not disable or block it. The paint walk
// draws it dimmed while the grid is disabled, as it does any content under a
// disabled widget (belluikit/bellui#1393).
//
// Disable is safe before the grid is drawn.
func (g *DataGrid) Disable() {
	if g.disabled {
		return
	}
	g.disabled = true

	editor := g.editWidget
	g.editingCell = nil
	g.editWidget = nil
	g.editFactory = nil
	g.editOldValue = nil

	g.dismissTooltip()
	g.hoveredCell = nil
	g.nearResizeBorder = false
	g.mousePressed = false
	g.resizeCol = -1
	g.reorderFromCol = -1
	g.dragRow = -1
	g.dragCol = -1
	g.swipe.gesture = gestureUndecided
	g.closeSwipe(false)

	// The body scrolls through a container.Scroll inside the grid, which the
	// wheel and the scroll bars reach directly. ScrollNone stops both and
	// hides the bars; the offset is kept.
	g.scrollDir = g.scrollContent.Scroll.Direction
	g.scrollContent.Scroll.Direction = container.ScrollNone

	g.dropFocus(editor)
	g.refreshParts()
}

// Enable undoes Disable: the grid takes input again and draws its normal
// look. A focus Disable dropped does not come back.
func (g *DataGrid) Enable() {
	if !g.disabled {
		return
	}
	g.disabled = false
	g.scrollContent.Scroll.Direction = g.scrollDir
	g.refreshParts()
}

// Disabled reports whether the grid is disabled.
func (g *DataGrid) Disabled() bool {
	return g.disabled
}

// dropFocus unfocuses the canvas when its focus is on the grid or on a part
// the grid builds: a filter entry, a select checkbox, or editor, the inline
// editor Disable just closed. The toolkit's own Disable drops a focus inside
// a widget by walking its tree, which an x module cannot do; these are the
// grid's focusable parts.
func (g *DataGrid) dropFocus(editor core.CanvasObject) {
	if !g.rendered {
		return // never drawn: nothing in it can hold a focus
	}
	a := bellui.CurrentApp()
	if a == nil {
		return
	}
	c := a.Driver().CanvasForObject(g)
	if c == nil {
		return
	}
	f := c.Focused()
	if f == nil {
		return
	}
	mine := any(f) == any(g) ||
		(editor != nil && any(f) == any(editor)) ||
		g.cellsWidget.HoldsFocusable(f) ||
		(g.floatingFilterRow != nil && g.floatingFilterRow.HoldsFocusable(f))
	if mine {
		c.Unfocus()
	}
}

// refreshParts repaints the grid and each band it draws, so the disabled
// look and the parts' disabled state apply at once: a grid Refresh lays the
// bands out but does not refresh every one of them.
func (g *DataGrid) refreshParts() {
	g.Refresh()
	g.headerWidget.Refresh()
	g.cellsWidget.Refresh()
	g.pinnedLeftCells.Refresh()
	g.pinnedRightCells.Refresh()
	g.groupRowOverlay.Refresh()
	if g.floatingFilterRow != nil {
		g.floatingFilterRow.Refresh()
	}
	g.scrollContent.Scroll.Refresh()
}

// dimBuiltinCell draws a cell of a renderer the grid picked from the column
// type in the disabled colour while the grid is disabled. The built-in text,
// number and date renderers set the foreground colour on every update, so the
// cell gets its colour back on the first update after Enable. A Boolean cell
// is a widget.Check the renderer always disables. A column's own
// CellRenderer is app content and is left alone.
func (g *DataGrid) dimBuiltinCell(rc *resolvedColumn, obj core.CanvasObject) {
	if !g.disabled || !rc.builtin {
		return
	}
	if t, ok := obj.(*canvas.Text); ok {
		t.Color = theme.Color(theme.ColorNameDisabled)
		t.Refresh()
	}
}
