package widget

import (
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// Mobile touch text selection for the read-only selectable surfaces —
// Label.Selectable / RichText.Selectable (pre-rebrand tracker #944, the pre-rebrand tracker #758 follow-up). A
// hold-fired long-press selects the word under the finger and summons the
// shared selection-handle overlay (entry_handles.go); the release then still
// opens the Copy context menu via TappedSecondary, per the hold-fire
// contract in internal/driver/common/touch.go. Everything is gated to
// mobile devices, so desktop behavior and render masters are unchanged.

var (
	_ event.LongPressable      = (*selectable)(nil)
	_ selectionHandleHost      = (*selectable)(nil)
	_ selectionLoupeStyledHost = (*selectable)(nil)
)

// LongPressed implements [event.LongPressable]. A touch held on a selectable
// places the cursor at the press point and selects the word under it — the
// native mobile gesture. Desktop right-click is unaffected (it dispatches
// TappedSecondary directly).
func (s *selectable) LongPressed(ev *event.PointEvent) {
	if s.host != nil {
		// An embedding editor (RichEntry) owns pointer gestures; forward when
		// it takes long-press, otherwise do nothing — never drive the
		// selectable's own model under a host (mirrors Tapped/Dragged).
		if lp, ok := s.host.(event.LongPressable); ok {
			lp.LongPressed(ev)
		}
		return
	}

	s.grabFocus()
	s.cursorRow, s.cursorColumn, _ = s.getRowCol(ev.Position)
	if s.password {
		// Concealed text must not reveal word boundaries; the release still
		// opens the context menu.
		return
	}
	s.selectWordAtCursor()
	s.markTouchSelection()
}

// selectWordAtCursor selects the word under the cursor — snapping to the
// nearest adjacent word when the press lands on whitespace (pre-rebrand tracker #958) — anchoring
// the selection at its start; the selectable analog of Entry's long-press
// selection (selectNearestWordAtCursor).
func (s *selectable) selectWordAtCursor() {
	row := s.provider.row(s.cursorRow)
	start, end := getTextWordRegionNearest(row, s.cursorColumn)
	if start == -1 || end == -1 {
		return
	}

	s.selectRow = s.cursorRow
	s.selectColumn = start
	s.cursorColumn = end
	s.selecting = true
	s.Refresh()
	s.notifySelectChanged()
}

// markTouchSelection flags the current selection as touch-initiated and
// summons the selection handles for it (a no-op off mobile, and for a
// selectable embedded in an editor).
func (s *selectable) markTouchSelection() {
	if !s.ownsSelectionHandles() {
		return
	}
	s.touchSelection = true
	s.updateSelectionHandles()
}

// ownsSelectionHandles reports whether this selectable drives its own mobile
// selection handles. Only the standalone read-only selectable behind
// Label/RichText does: when embedded in an editor — Entry's e.sel, or
// RichEntry via host — the editor owns the gestures and handle lifecycle.
func (s *selectable) ownsSelectionHandles() bool {
	if s.host != nil {
		return false
	}
	obj, ok := s.focus.(core.CanvasObject)
	return ok && obj == s.super()
}

// updateSelectionHandles shows, repositions or hides the selection handles
// for a standalone selectable, mirroring Entry.updateSelectionHandles: the
// renderer's Refresh is the central seam, and the overlay's per-frame sync
// animation tracks movement the selectable doesn't know about.
func (s *selectable) updateSelectionHandles() {
	if !s.ownsSelectionHandles() || !bellui.CurrentDevice().IsMobile() {
		return
	}

	if !s.selecting {
		s.touchSelection = false // the touch-initiated selection is gone
	}
	show := s.touchSelection && s.focussed && s.Visible() && !s.password
	if show {
		a, _ := s.selection()
		show = a >= 0
	}
	syncSelectionHandles(s, &s.selHandles, show)
}

// The selectable's selectionHandleHost implementation. The selectable is a
// full-size sibling of its RichText provider (both at the widget origin), so
// its local coordinate space is the provider's content space and there is no
// scroll offset.

func (s *selectable) selectionHandleObject() core.CanvasObject {
	return s.super()
}

// selectionHandleEdges returns the selection edges in (row, column) space,
// anchor/cursor order independent (the selection() convention).
func (s *selectable) selectionHandleEdges() (rowA, colA, rowB, colB int) {
	rowA, colA = s.selectRow, s.selectColumn
	rowB, colB = s.cursorRow, s.cursorColumn
	if rowB < rowA || (rowB == rowA && colB < colA) {
		rowA, colA, rowB, colB = rowB, colB, rowA, colA
	}
	return rowA, colA, rowB, colB
}

// selectionHandleCaretRect computes the caret cell at (row, col) from the
// provider's row geometry — the same measurements the selection highlight
// uses (selectableRenderer.buildSelection), so a handle sits exactly under
// its highlight edge.
func (s *selectable) selectionHandleCaretRect(row, col int) geo.Rectangle {
	prov := s.provider
	th := s.theme
	textSize := th.Size(s.getSizeName())
	innerPad := th.Size(theme.SizeNameInnerPadding)

	sz := prov.lineSizeToColumn(col, row, textSize, innerPad, true)
	x := sz.Width
	// A row with an inline image atom counts columns in document-position
	// space; use the atom-aware walk for the x when available (pre-rebrand tracker #893).
	if ix, ok := prov.inlineColumnX(row, col, textSize, innerPad); ok {
		x = ix
	}
	x += s.alignOffsetX(row) + prov.GutterWidth

	y, rh := prov.RowGeometry(row)
	if rh <= 0 {
		rh = prov.charMinSize(s.password, s.style, textSize).Height
	}
	if y == 0 && row > 0 {
		y = rh * float32(row) // uniform fallback before the first layout
	}

	// One character cell of width; at end-of-line the next-column measurement
	// does not advance, so fall back to the monospace char width.
	w := prov.lineSizeToColumn(col+1, row, textSize, innerPad, true).Width - sz.Width
	if w <= 0 {
		w = prov.charMinSize(s.password, s.style, textSize).Width
	}
	return geo.NewRectangle(x, y, w, rh)
}

func (s *selectable) selectionHandleScroll() geo.Position {
	return geo.Position{} // a read-only selectable does not scroll its content
}

func (s *selectable) selectionHandleLineHeight() float32 {
	textSize := s.theme.Size(s.getSizeName())
	return s.provider.charMinSize(s.password, s.style, textSize).Height
}

// dragSelectionHandle moves one selection edge to a dragged content
// position. The opposite edge is pinned and the dragged edge is clamped so
// the selection stays non-empty — the handles cannot cross or collapse it.
// Unlike Entry the clamp works purely in (row, column) space: the read-only
// selectable has no caret model, and rune offsets are unreliable across
// multi-segment RichText rows.
func (s *selectable) dragSelectionHandle(isStart bool, content geo.Position) {
	if !s.selecting {
		return // selection vanished under us; the next refresh hides the overlay
	}

	rowA, colA, rowB, colB := s.selectionHandleEdges()
	dRow, dCol, _ := s.getRowCol(content)

	var aRow, aCol int
	ok := true
	if isStart {
		aRow, aCol = rowB, colB // the end edge stays pinned as the anchor
		if !rowColBefore(dRow, dCol, aRow, aCol) {
			dRow, dCol, ok = s.positionBefore(aRow, aCol)
		}
	} else {
		aRow, aCol = rowA, colA // the start edge stays pinned as the anchor
		if !rowColBefore(aRow, aCol, dRow, dCol) {
			dRow, dCol, ok = s.positionAfter(aRow, aCol)
		}
	}
	if !ok {
		return // no valid position keeps the selection non-empty
	}

	changed := s.cursorRow != dRow || s.cursorColumn != dCol ||
		s.selectRow != aRow || s.selectColumn != aCol
	s.selectRow, s.selectColumn = aRow, aCol
	s.cursorRow, s.cursorColumn = dRow, dCol
	s.touchSelection = true
	if changed {
		s.notifySelectChanged()
		s.Refresh() // repaints the highlight and re-syncs the handles
	}
}

// The selectable's selectionLoupeHost implementation (pre-rebrand tracker #942, adopted with the
// pre-rebrand tracker #888 parity work): the drag magnifier over a selectable Label/RichText
// renders the dragged row as styled runs, so multi-styled RichText rows
// magnify true to their drawn widths.

func (s *selectable) loupeRowText(row int) string {
	text := string(s.provider.row(row))
	if s.password {
		return strings.Repeat(passwordChar, len([]rune(text)))
	}
	return text
}

func (s *selectable) loupeRowVisualX(row, col int) float32 {
	return loupeProviderVisualX(s.provider, row, col, s.theme, s.getSizeName())
}

func (s *selectable) loupeTextStyle() bellui.TextStyle {
	return s.style
}

func (s *selectable) loupeRowRuns(row int) []canvas.TextRun {
	return loupeRunsForRow(s.provider, row)
}

// rowColBefore reports whether (r1, c1) precedes (r2, c2) in document order.
func rowColBefore(r1, c1, r2, c2 int) bool {
	return r1 < r2 || (r1 == r2 && c1 < c2)
}

// positionBefore returns the (row, col) one position before the given one in
// document order, stepping over row boundaries; ok is false at the start of
// the document. (On a soft-wrapped boundary the previous row's end shares a
// rune offset with this row's start — an acceptable transient while the
// finger sits exactly on the anchor.)
func (s *selectable) positionBefore(row, col int) (r, c int, ok bool) {
	if col > 0 {
		return row, col - 1, true
	}
	if row > 0 {
		return row - 1, s.provider.rowLength(row - 1), true
	}
	return 0, 0, false
}

// positionAfter returns the (row, col) one position after the given one in
// document order; ok is false at the end of the document.
func (s *selectable) positionAfter(row, col int) (r, c int, ok bool) {
	if col < s.provider.rowLength(row) {
		return row, col + 1, true
	}
	if row < s.provider.rows()-1 {
		return row + 1, 0, true
	}
	return 0, 0, false
}
