package widget

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/theme"
)

// Mobile touch text selection for RichEntry (pre-rebrand tracker #888, parity with Entry's pre-rebrand tracker #758
// and the selectable's pre-rebrand tracker #944): a hold-fired long-press selects the word under
// the finger and summons the shared selection-handle overlay
// (entry_handles.go); dragging a handle adjusts the selection with the drag
// magnifier (pre-rebrand tracker #942) over the target; the context menu re-appears on lift.
// RichEntry adopts the same selectionHandleHost / selectionLoupeHost seams
// Entry and the selectable implement, mapped onto its offset-based document
// model. Everything is gated to mobile devices, so desktop behavior and
// render masters are unchanged.

var (
	_ event.LongPressable      = (*RichEntry)(nil)
	_ event.SecondaryTappable  = (*RichEntry)(nil)
	_ mobile.Touchable         = (*RichEntry)(nil)
	_ desktop.Mouseable        = (*RichEntry)(nil)
	_ selectionHandleHost      = (*RichEntry)(nil)
	_ selectionLoupeHost       = (*RichEntry)(nil)
	_ selectionLoupeStyledHost = (*RichEntry)(nil)
)

// LongPressed implements [event.LongPressable]. A touch held on the editor
// places the caret at the press point and selects the word under it — the
// native mobile gesture; the release that follows opens the context menu via
// TappedSecondary. Desktop right-click is unaffected (it dispatches
// TappedSecondary directly). The embedded selectable forwards the driver's
// dispatch here (it is the deeper LongPressable in the object tree).
func (e *RichEntry) LongPressed(ev *event.PointEvent) {
	e.requestFocus()
	e.syncSel()
	row, col, _ := e.sel.getRowCol(ev.Position)
	off := clampInt(e.doc.rowColToOffset(row, col), 0, e.doc.len())
	e.caret, e.anchor, e.selecting = off, off, false

	// Nearest-word snap: a press on whitespace selects the adjacent word, not
	// the whitespace run (pre-rebrand tracker #958, matching Entry/selectable long-press).
	start, end := getTextWordRegionNearest(e.provider.row(row), col)
	if start != -1 && end != -1 {
		e.anchor = e.doc.rowColToOffset(row, start)
		e.caret = e.doc.rowColToOffset(row, end)
		e.selecting = e.anchor != e.caret
	}
	e.interruptCursor()
	if e.selecting {
		e.markTouchSelection()
	}
	e.Refresh()
}

// TouchDown records that the active pointer is a touch, which gates the
// selection handles to touch-initiated selections. The embedded selectable
// forwards its TouchDown here (like the other pointer gestures).
func (e *RichEntry) TouchDown(*mobile.TouchEvent) {
	e.touchPointer = true
}

// TouchUp implements [mobile.Touchable].
func (e *RichEntry) TouchUp(*mobile.TouchEvent) {
}

// TouchCancel implements [mobile.Touchable].
func (e *RichEntry) TouchCancel(*mobile.TouchEvent) {
}

// MouseDown records that the active pointer is a mouse (forwarded by the
// embedded selectable, which grabs focus itself).
func (e *RichEntry) MouseDown(*desktop.MouseEvent) {
	e.touchPointer = false
}

// MouseUp implements [desktop.Mouseable].
func (e *RichEntry) MouseUp(*desktop.MouseEvent) {
}

// TappedSecondary opens the editor's context menu (Undo/Redo when available,
// Cut, Copy, Paste, Select all) at the tapped position. On mobile it is also
// re-summoned after a long-press release or a handle adjustment — the native
// "text toolbar reappears" behavior (pre-rebrand tracker #945).
func (e *RichEntry) TappedSecondary(pe *event.PointEvent) {
	e.requestFocus()
	pos := bellui.CurrentApp().Driver().AbsolutePositionForObject(e.super()).Add(pe.Position)
	e.showContextMenu(pos)
}

// showContextMenu builds and presents the editor's context menu at the given
// absolute canvas position.
func (e *RichEntry) showContextMenu(pos geo.Position) {
	super := e.super()
	app := bellui.CurrentApp()
	clipboard := app.Clipboard()
	c := app.Driver().CanvasForObject(super)
	if c == nil {
		return
	}

	menuItems := make([]*bellui.MenuItem, 0, 7)
	canUndo, canRedo := e.undo.canUndo(), e.undo.canRedo()
	if canUndo {
		menuItems = append(menuItems, bellui.NewMenuItem(lang.L("Undo"), e.Undo))
	}
	if canRedo {
		menuItems = append(menuItems, bellui.NewMenuItem(lang.L("Redo"), e.Redo))
	}
	if canUndo || canRedo {
		menuItems = append(menuItems, bellui.NewMenuItemSeparator())
	}
	menuItems = append(menuItems,
		bellui.NewMenuItem(lang.L("Cut"), func() {
			e.cutToClipboard(clipboard)
		}),
		bellui.NewMenuItem(lang.L("Copy"), func() {
			e.copyToClipboard(clipboard)
		}),
		bellui.NewMenuItem(lang.L("Paste"), func() {
			e.pasteFromClipboard(clipboard)
		}),
		bellui.NewMenuItem(lang.L("Select all"), func() {
			e.selectAll()
			if e.touchPointer {
				e.markTouchSelection() // Select all from the touch menu gets handles
				// Re-present the menu over the now-full selection so Cut/Copy
				// stay reachable (native toolbar parity, pre-rebrand tracker #1128); the current
				// menu is dismissing as part of this tap.
				bellui.Do(func() { e.showContextMenu(pos) })
			}
		}))

	pop := NewPopUpMenu(bellui.NewMenu("", menuItems...), c)
	inheritDirection(pop, super) // #749
	pop.ShowAtPosition(pos)
}

// markTouchSelection flags the current selection as touch-initiated and
// summons the selection handles for it (a no-op off mobile). The caret handle
// (pre-rebrand tracker #999) is dismissed first: both are sibling transient overlays and
// OverlayStack.Remove drops the target plus everything stacked above it, so
// removing the older caret handle after the newer selection handles were added
// would take the selection handles with it. Hiding it before they are added
// keeps the transition clean (matching Entry, where Select() refreshes the
// caret handle away before markTouchSelection adds the selection handles).
func (e *RichEntry) markTouchSelection() {
	e.touchSelection = true
	e.hideCaretHandle()
	e.updateSelectionHandles()
}

// updateSelectionHandles shows, repositions or hides the selection handles.
// It is the central seam: every selection mutation funnels through the
// editor's Refresh, which calls it; the overlay's per-frame sync animation
// tracks movement the editor doesn't know about (a parent container
// scrolling).
func (e *RichEntry) updateSelectionHandles() {
	if !bellui.CurrentDevice().IsMobile() {
		return
	}

	if _, _, ok := e.selection(); !ok || e.composing {
		e.touchSelection = false // the touch-initiated selection is gone
	}
	show := e.touchSelection && e.focused && e.Visible()
	syncSelectionHandles(e, &e.selHandles, show)
}

// RichEntry's selectionHandleHost implementation: the overlay's (row, column)
// space maps through the document position model (offsets in which each image
// atom counts one position), so the handles work over styled runs, images and
// soft-wrapped rows alike.

func (e *RichEntry) selectionHandleObject() core.CanvasObject {
	return e.super()
}

// selectionHandleEdges returns the selection edges in (row, column) space,
// anchor/caret order independent (the selection() convention).
func (e *RichEntry) selectionHandleEdges() (rowA, colA, rowB, colB int) {
	from, to := e.anchor, e.caret
	if from > to {
		from, to = to, from
	}
	rowA, colA = e.doc.offsetToRowCol(from)
	rowB, colB = e.doc.offsetToRowCol(to)
	return rowA, colA, rowB, colB
}

func (e *RichEntry) selectionHandleCaretRect(row, col int) geo.Rectangle {
	pos, size := e.caretRectAt(row, col, e.doc.rowIsImage(row))
	return geo.NewRectangle(pos.X, pos.Y, size.Width, size.Height)
}

func (e *RichEntry) selectionHandleScroll() geo.Position {
	return geo.Position{} // the editor does not scroll its own content
}

func (e *RichEntry) selectionHandleLineHeight() float32 {
	return e.provider.charMinSize(false, bellui.TextStyle{}, e.Theme().Size(theme.SizeNameText)).Height
}

// dragSelectionHandle moves one selection edge to a dragged content position.
// The dragged edge carries the caret; the opposite edge is pinned as the
// anchor and the selection is clamped to stay non-empty, so the handles
// cannot cross or collapse it. The clamp works in offset space — the
// document position model in which an image atom is one position.
func (e *RichEntry) dragSelectionHandle(isStart bool, content geo.Position) {
	from, to, ok := e.selection()
	if !ok {
		return // selection vanished under us; the next refresh hides the overlay
	}

	e.syncSel()
	row, col, _ := e.sel.getRowCol(content)
	dragOff := clampInt(e.doc.rowColToOffset(row, col), 0, e.doc.len())

	var anchorOff int
	if isStart {
		anchorOff = to
		if dragOff >= anchorOff {
			dragOff = anchorOff - 1 // keep at least one position selected
		}
	} else {
		anchorOff = from
		if dragOff <= anchorOff {
			dragOff = anchorOff + 1
		}
	}
	if dragOff < 0 || dragOff > e.doc.len() {
		return
	}

	changed := e.caret != dragOff || e.anchor != anchorOff
	e.anchor, e.caret, e.selecting = anchorOff, dragOff, true
	e.touchSelection = true
	if changed {
		e.interruptCursor()
		e.Refresh() // repaints the highlight and re-syncs the handles
	}
}

// RichEntry's selectionLoupeHost implementation (pre-rebrand tracker #942): the drag magnifier
// renders the dragged row as styled runs (loupeRowRuns), so bold/heading runs
// magnify true to their drawn widths. Rows mixing inline image atoms magnify
// their text only — the caret marker stays truthful to the real edge, the
// text copy may sit offset around the atom.

func (e *RichEntry) loupeRowText(row int) string {
	return string(e.provider.row(row))
}

func (e *RichEntry) loupeRowVisualX(row, col int) float32 {
	return loupeProviderVisualX(&e.provider, row, col, e.Theme(), theme.SizeNameText)
}

func (e *RichEntry) loupeTextStyle() bellui.TextStyle {
	return bellui.TextStyle{}
}

func (e *RichEntry) loupeRowRuns(row int) []canvas.TextRun {
	return loupeRunsForRow(&e.provider, row)
}
