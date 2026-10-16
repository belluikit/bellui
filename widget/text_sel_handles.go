package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/theme"
)

// selectionHandleHost is the seam between a text widget and the mobile
// selection-handle overlay (pre-rebrand tracker #758, generalized in pre-rebrand tracker #944). Entry, the
// standalone read-only selectable (Label.Selectable / RichText.Selectable)
// and RichEntry (pre-rebrand tracker #888) implement it — the interface deliberately exposes
// only what the overlay needs (selection edges and caret geometry in the
// host's own (row, column) space, a scroll offset, and an apply-drag hook),
// so an editor with its own document model can join without sharing the
// selectable's internals.
type selectionHandleHost interface {
	// selectionHandleObject is the host object on the canvas: the anchor for
	// CanvasForObject/AbsolutePositionForObject and the viewport whose
	// bounds cull off-screen handles.
	selectionHandleObject() core.CanvasObject
	// Theme sizes the handle dots.
	Theme() bellui.Theme

	// selectionHandleEdges returns the selection edges in (row, column)
	// space, normalized to document order (edge A never after edge B),
	// independent of which edge carries the caret/anchor.
	selectionHandleEdges() (rowA, colA, rowB, colB int)
	// selectionHandleCaretRect returns the caret cell at (row, col) in the
	// host's content coordinate space (pre-scroll).
	selectionHandleCaretRect(row, col int) geo.Rectangle
	// selectionHandleScroll is the host's current content scroll offset;
	// zero for hosts that do not scroll their content.
	selectionHandleScroll() geo.Position
	// selectionHandleLineHeight is the height of one text line, used to lift
	// the finger position from the handle back into the line it controls.
	selectionHandleLineHeight() float32

	// dragSelectionHandle moves the selection edge a handle drag controls
	// (start true = the leading edge) to the content position, pinning the
	// opposite edge and clamping so the selection stays non-empty, then
	// refreshes the host.
	dragSelectionHandle(start bool, contentPos geo.Position)

	// updateSelectionHandles re-evaluates gating and geometry: it shows,
	// repositions or hides the overlay (both the per-frame sync tick and
	// the host's refresh seam funnel here).
	updateSelectionHandles()
}

// hostAbsolutePosition returns a host object's canvas-absolute origin.
// Driver.AbsolutePositionForObject reports positions relative to the canvas
// interactive area (the mobile driver subtracts the display inset), while
// overlay children and touch events use canvas-absolute coordinates — a
// mismatch the pre-rebrand tracker #945 device pass exposed (handles drawn one status-bar too
// high, handle drags landing one status-bar too low). Adding the
// interactive-area origin back restores canvas-absolute; a no-op on
// inset-free drivers. The test driver's deliberate non-zero inset keeps
// this honest in the host tests.
func hostAbsolutePosition(obj core.CanvasObject) geo.Position {
	driver := bellui.CurrentApp().Driver()
	pos := driver.AbsolutePositionForObject(obj)
	if cnv := driver.CanvasForObject(obj); cnv != nil {
		origin, _ := cnv.InteractiveArea()
		pos = pos.Add(origin)
	}
	return pos
}

// syncSelectionHandles is the shared show/hide seam behind each host's
// updateSelectionHandles: given whether the host currently wants handles it
// creates, repositions or hides the overlay stored at h. show is forced off
// while the host is not on a canvas. h is the host's own storage field, by
// pointer, because it must be assigned *before* showOn runs: the per-frame
// sync animation can tick synchronously inside Start (the test driver does),
// re-entering updateSelectionHandles — which must find the overlay already
// stored or it would create another, without limit.
func syncSelectionHandles(host selectionHandleHost, h **entrySelectionHandles, show bool) {
	var cnv core.Canvas
	if show {
		if app := bellui.CurrentApp(); app != nil && app.Driver() != nil {
			cnv = app.Driver().CanvasForObject(host.selectionHandleObject())
		}
		show = cnv != nil
	}

	if !show {
		if *h != nil {
			(*h).hide()
		}
		return
	}
	if *h == nil {
		*h = newEntrySelectionHandles(host)
	}
	(*h).showOn(cnv)
}

// Entry's selectionHandleHost implementation (pre-rebrand tracker #758). The gating half lives
// in Entry.updateSelectionHandles (entry_handles.go).

var _ selectionHandleHost = (*Entry)(nil)

func (e *Entry) selectionHandleObject() core.CanvasObject {
	return e.super()
}

// selectionHandleEdges returns the selection edges in (row, column) space,
// anchor/caret order independent (the selection() convention).
func (e *Entry) selectionHandleEdges() (rowA, colA, rowB, colB int) {
	rowA, colA = e.sel.selectRow, e.sel.selectColumn
	rowB, colB = e.CursorRow, e.CursorColumn
	if rowB < rowA || (rowB == rowA && colB < colA) {
		rowA, colA, rowB, colB = rowB, colB, rowA, colA
	}
	return rowA, colA, rowB, colB
}

func (e *Entry) selectionHandleCaretRect(row, col int) geo.Rectangle {
	return e.caretRect(row, col, true)
}

func (e *Entry) selectionHandleScroll() geo.Position {
	if e.scroll == nil {
		return geo.Position{}
	}
	return e.scroll.Offset
}

func (e *Entry) selectionHandleLineHeight() float32 {
	th := e.Theme()
	return e.text.charMinSize(false, e.TextStyle, th.Size(theme.SizeNameText)).Height
}

// dragSelectionHandle moves one selection edge to a dragged content
// position. The dragged edge carries the caret (so ensureCursorVisible
// auto-scrolls the entry); the opposite edge is pinned as the anchor and the
// selection is clamped to stay non-empty, so the handles cannot cross or
// collapse it.
func (e *Entry) dragSelectionHandle(isStart bool, content geo.Position) {
	e.syncSelectable()

	startOff, endOff := e.sel.selection()
	if startOff < 0 {
		return // selection vanished under us; the next refresh hides the overlay
	}

	row, col, _ := e.sel.getRowCol(content)
	prov := e.textProvider()
	dragOff := textPosFromRowCol(row, col, prov)

	var anchorOff int
	if isStart {
		anchorOff = endOff
		if dragOff >= anchorOff {
			dragOff = anchorOff - 1 // keep at least one rune selected
		}
	} else {
		anchorOff = startOff
		if dragOff <= anchorOff {
			dragOff = anchorOff + 1
		}
	}
	if dragOff < 0 || dragOff > prov.len() {
		return
	}

	aRow, aCol := rowColFromTextPos(anchorOff, prov, e.Wrapping)
	dRow, dCol := rowColFromTextPos(dragOff, prov, e.Wrapping)
	changed := e.CursorRow != dRow || e.CursorColumn != dCol ||
		e.sel.selectRow != aRow || e.sel.selectColumn != aCol
	e.sel.selectRow, e.sel.selectColumn = aRow, aCol
	e.CursorRow, e.CursorColumn = dRow, dCol
	e.syncSelectable()
	e.sel.selecting = true
	e.touchSelection = true
	if changed {
		e.sel.notifySelectChanged()
		e.Refresh() // repaints the highlight, auto-scrolls, re-syncs the handles
	}
}
