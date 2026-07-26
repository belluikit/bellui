//go:build mobile

package widget

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
)

// The mobile caret drag-handle for RichEntry (#999, parity with Entry's #943):
// a plain touch tap placing the caret shows a single draggable dot below the
// insertion point; dragging it moves the caret without selecting; it hides on
// typing, focus loss, selection start and after an inactivity timeout.

// touchTapRich places the caret with a touch, following the mobile driver's
// dispatch order for a tap: TouchDown, TouchUp, then Tapped.
func touchTapRich(e *RichEntry, pos geo.Position) {
	e.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: pos}})
	e.TouchUp(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: pos}})
	e.Tapped(&event.PointEvent{Position: pos})
}

func richCaretHandleShown(e *RichEntry, c core.Canvas) bool {
	if e.caretHandle == nil {
		return false
	}
	for _, o := range c.Overlays().List() {
		if o == e.caretHandle {
			return true
		}
	}
	return false
}

// caretPos is the content-space position of the caret at (row, col).
func caretPos(e *RichEntry, row, col int) geo.Position {
	pos, _ := e.caretRectAt(row, col, e.doc.rowIsImage(row))
	return pos
}

func TestRichEntry_CaretHandle_ShowsOnTouchTap(t *testing.T) {
	e, c := showHandlesRichEntry(t)
	require.Empty(t, c.Overlays().List())

	touchTapRich(e, caretPos(e, 0, 2))
	require.NotNil(t, e.caretHandle)
	assert.True(t, richCaretHandleShown(e, c), "caret handle overlay shown after a touch tap")
	assert.True(t, e.caretHandle.grab.Visible())

	// The dot hangs below the caret, horizontally centered on it.
	row, col := e.doc.offsetToRowCol(e.caret)
	pos, size := e.caretRectAt(row, col, e.doc.rowIsImage(row))
	anchor := pos.Add(hostAbsolutePosition(e.super()).Subtract(e.caretHandle.Position()))
	grab := e.caretHandle.grab
	assert.InDelta(t, anchor.X, grab.Position().X+grab.Size().Width/2, 0.5)
	assert.InDelta(t, anchor.Y+size.Height, grab.Position().Y, 0.5)
}

func TestRichEntry_CaretHandle_DragMovesCaretWithoutSelecting(t *testing.T) {
	e, c := showHandlesRichEntry(t)
	touchTapRich(e, caretPos(e, 0, 0))
	require.True(t, richCaretHandleShown(e, c))
	require.Equal(t, 0, e.caret)

	// Drag the dot under column 11 of the first line.
	grab := e.caretHandle.grab
	pos, size := e.caretRectAt(0, 11, false)
	fingerOverlay := pos.
		Add(hostAbsolutePosition(e.super()).Subtract(e.caretHandle.Position())).
		AddXY(0, size.Height+grab.Size().Height/2) // finger on the dot below the line
	grab.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: fingerOverlay.Subtract(grab.Position())}})
	grab.DragEnd()

	assert.Equal(t, e.doc.rowColToOffset(0, 11), e.caret, "dragging the caret handle moves the caret")
	_, _, hasSel := e.selection()
	assert.False(t, hasSel, "a caret-handle drag never creates a selection")
	assert.True(t, richCaretHandleShown(e, c), "the handle stays up while being dragged")
}

func TestRichEntry_CaretHandle_HidesOnTyping(t *testing.T) {
	e, c := showHandlesRichEntry(t)

	touchTapRich(e, caretPos(e, 0, 2))
	require.True(t, richCaretHandleShown(e, c))
	e.TypedRune('x')
	assert.False(t, richCaretHandleShown(e, c), "typing a rune dismisses the caret handle")

	touchTapRich(e, caretPos(e, 0, 2))
	require.True(t, richCaretHandleShown(e, c))
	e.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.False(t, richCaretHandleShown(e, c), "a key press dismisses the caret handle")
}

func TestRichEntry_CaretHandle_HidesOnFocusLoss(t *testing.T) {
	e, c := showHandlesRichEntry(t)

	touchTapRich(e, caretPos(e, 0, 2))
	require.True(t, richCaretHandleShown(e, c))

	c.Unfocus()
	assert.False(t, richCaretHandleShown(e, c), "focus loss dismisses the caret handle")

	// Re-focusing without a touch must not resurrect it.
	c.Focus(e)
	e.Refresh()
	assert.False(t, richCaretHandleShown(e, c), "only a touch tap re-arms the handle")
}

func TestRichEntry_CaretHandle_InactivityTimeout(t *testing.T) {
	e, c := showHandlesRichEntry(t)

	touchTapRich(e, caretPos(e, 0, 2))
	require.True(t, richCaretHandleShown(e, c))

	// The per-frame sync enforces the timeout; simulate its next tick after
	// the deadline instead of sleeping.
	e.caretHandle.shownAt = time.Now().Add(-richEntryCaretHandleTimeout - time.Second)
	e.updateCaretHandle()
	assert.False(t, richCaretHandleShown(e, c), "the caret handle times out after inactivity")
	assert.False(t, e.caretHandle.active)
}

func TestRichEntry_CaretHandle_SelectionHandlesTakeOver(t *testing.T) {
	e, c := showHandlesRichEntry(t)

	touchTapRich(e, caretPos(e, 0, 2))
	require.True(t, richCaretHandleShown(e, c))

	longPressRichEntry(e, 0, 2) // selects "hello", a touch-initiated selection
	require.NotEmpty(t, e.SelectedText())
	assert.False(t, richCaretHandleShown(e, c), "a selection dismisses the caret handle")
	assert.Contains(t, c.Overlays().List(), e.selHandles, "the selection handles take over")
}
