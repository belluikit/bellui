package render

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/widget"
)

// cellHost wraps a realized cell's content object so it is an accessible
// semantic node carrying the cell's role, label, coordinates and selection
// state — mirroring fyne's widget.Table tableCellHost. The default cell
// content is a canvas.Text (not a core.Accessible), so without this wrapper a
// screen reader would see the table's cells as unlabeled, position-less blobs.
//
// It is deliberately not Tappable/Hoverable: the DataGrid's own geometry-based
// hit-testing keeps handling input. Each host owns one content object for its
// lifetime (1:1, like tableCellHost) and is recycled via its renderer's own host
// pool. Both the main renderer (CellsRenderer) and the pinned renderer
// (PinnedCellsRenderer) wrap their cells in this host, so pinned and main cells
// carry identical grid a11y semantics.
//
// Coordinates are display coordinates in current presentation order, in the
// ABSOLUTE column space (pinned-left + main + pinned-right, left to right) that
// AccessibleGridDimensions also reports — the visual/current-view frame
// documented on core.AccessibleGridItem — so they change on sort/filter. Stable
// source identity travels on a separate channel (see the AccessibleIdentified
// follow-up, fyne pre-rebrand tracker #635); it is intentionally not this position.
type cellHost struct {
	widget.BaseWidget

	grid     GridAccess
	content  core.CanvasObject
	row, col int
}

var (
	_ core.Accessible           = (*cellHost)(nil)
	_ core.AccessibleGridItem   = (*cellHost)(nil)
	_ core.AccessibleStateful   = (*cellHost)(nil)
	_ core.AccessibleIdentified = (*cellHost)(nil)
	_ core.AccessibleActioner   = (*cellHost)(nil)
)

func newCellHost(grid GridAccess, content core.CanvasObject) *cellHost {
	h := &cellHost{grid: grid, content: content}
	h.ExtendBaseWidget(h)
	return h
}

func (h *cellHost) CreateRenderer() core.WidgetRenderer {
	h.ExtendBaseWidget(h)
	return widget.NewSimpleRenderer(h.content)
}

// AccessibleRole returns core.RoleTableCell.
func (h *cellHost) AccessibleRole() core.AccessibleRole { return core.RoleTableCell }

// AccessibleLabel returns the cell's full (untruncated) text value.
func (h *cellHost) AccessibleLabel() string {
	return h.grid.AccessibleCellText(h.row, h.col)
}

// AccessibleCellPosition returns the cell's display row and column in current
// presentation order (see core.AccessibleGridItem). The checkbox-selection
// column is rendered at the start of the main (non-pinned) area — visually just
// right of the pinned-left band — so its reported column is that visual slot,
// PinnedLeftCount, matching what a sighted user sees (not absolute 0). Columns
// right of it (main + pinned-right) shift one place right; pinned-left columns
// keep their indices. The checkbox host carries the sentinel internal column
// -1, which maps to the slot.
//
// Internal lookups (label, state, identity) keep using h.col in the unshifted
// column space, so only the coordinate assistive tech sees is adjusted.
func (h *cellHost) AccessibleCellPosition() (row, col int) {
	col = h.col
	if h.grid.ShowCheckboxColumn() {
		pinnedLeft := h.grid.PinnedLeftCount()
		switch {
		case h.col < 0: // the checkbox host itself sits in its visual slot
			col = pinnedLeft
		case h.col >= pinnedLeft: // main + pinned-right shift right of the checkbox
			col = h.col + 1
		} // pinned-left columns (h.col < pinnedLeft) keep their index
	}
	return h.row, col
}

// AccessibleState reports StateSelected when the cell is selected and
// StateFocused when it is the grid's keyboard-focus cursor cell. The focused
// cell is also designated the grid's AT-focused descendant (see
// DataGrid.AccessibleActiveDescendant / pre-rebrand tracker #688); reporting StateFocused here
// keeps the node's own UIA focus property consistent with that designation.
func (h *cellHost) AccessibleState() core.AccessibleState {
	var st core.AccessibleState
	if h.grid.IsSelected(h.row, h.col) {
		st |= core.StateSelected
	}
	if h.grid.IsFocused(h.row, h.col) {
		st |= core.StateFocused
	}
	return st
}

// AccessibleIdentity returns the cell's stable, source-keyed identity so the
// platform bridges track this record across sort/scroll/filter rather than by
// the recycled host object (core.AccessibleIdentified). Nil for synthetic rows.
func (h *cellHost) AccessibleIdentity() any {
	return h.grid.AccessibleCellIdentity(h.row, h.col)
}

// AccessibleActions returns the "select" action (mirroring fyne's own
// tableCellHost). Declaring it is what makes the platform expose the cell's UIA
// SelectionItem pattern — carrying AccessibleState's StateSelected — and lets
// assistive tech select the cell; without it the bridge drops selection state
// on a RoleTableCell entirely.
func (h *cellHost) AccessibleActions() []core.AccessibleAction {
	return []core.AccessibleAction{{Name: core.ActionSelect, Description: "Select this cell"}}
}

// PerformAccessibleAction selects the cell like a pointer tap would.
func (h *cellHost) PerformAccessibleAction(name string) {
	if name == core.ActionSelect {
		h.grid.SelectCell(h.row, h.col)
	}
}
