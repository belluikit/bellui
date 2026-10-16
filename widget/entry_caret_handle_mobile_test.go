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
	"github.com/belluikit/bellui/test"
)

// The mobile caret drag-handle (pre-rebrand tracker #943): a plain touch tap placing the caret
// shows a single draggable dot below the insertion point; dragging it moves
// the caret without selecting; it hides on typing, focus loss, selection
// start and after an inactivity timeout.

// touchTap places the caret with a touch, following the mobile driver's
// dispatch order for a tap: TouchDown, TouchUp, then Tapped.
func touchTap(e *Entry, pos geo.Position) {
	e.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: pos}})
	e.TouchUp(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: pos}})
	e.Tapped(&event.PointEvent{Position: pos})
}

func caretHandleShown(e *Entry, c core.Canvas) bool {
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

func TestEntry_CaretHandle_ShowsOnTouchTap(t *testing.T) {
	e, c := showHandlesEntry(t)
	require.Empty(t, c.Overlays().List())

	touchTap(e, e.caretRect(0, 2, true).Pos.Subtract(e.scroll.Offset))
	require.NotNil(t, e.caretHandle)
	assert.True(t, caretHandleShown(e, c), "caret handle overlay shown after a touch tap")
	assert.True(t, e.caretHandle.grab.Visible())

	// The dot hangs below the caret, horizontally centered on it.
	rect := e.caretRect(e.CursorRow, e.CursorColumn, true)
	anchor := rect.Pos.Subtract(e.scroll.Offset).
		Add(hostAbsolutePosition(e).Subtract(e.caretHandle.Position()))
	grab := e.caretHandle.grab
	assert.InDelta(t, anchor.X, grab.Position().X+grab.Size().Width/2, 0.5)
	assert.InDelta(t, anchor.Y+rect.Size.Height, grab.Position().Y, 0.5)
}

func TestEntry_CaretHandle_DragMovesCaretWithoutSelecting(t *testing.T) {
	e, c := showHandlesEntry(t)
	touchTap(e, e.caretRect(0, 0, true).Pos.Subtract(e.scroll.Offset))
	require.True(t, caretHandleShown(e, c))
	require.Equal(t, 0, e.CursorColumn)

	// Drag the dot under "hello world"'s 11th column.
	grab := e.caretHandle.grab
	target := e.caretRect(0, 11, true) // content coords of the target caret
	fingerOverlay := target.Pos.Subtract(e.scroll.Offset).
		Add(hostAbsolutePosition(e).Subtract(e.caretHandle.Position())).
		AddXY(0, target.Size.Height+grab.Size().Height/2) // finger on the dot below the line
	grab.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: fingerOverlay.Subtract(grab.Position())}})
	grab.DragEnd()

	assert.Equal(t, 0, e.CursorRow)
	assert.Equal(t, 11, e.CursorColumn, "dragging the caret handle moves the caret")
	_, hasSel := e.SelectionRange()
	assert.False(t, hasSel, "a caret-handle drag never creates a selection")
	assert.True(t, caretHandleShown(e, c), "the handle stays up while being dragged")
}

func TestEntry_CaretHandle_HidesOnTyping(t *testing.T) {
	e, c := showHandlesEntry(t)

	touchTap(e, e.caretRect(0, 2, true).Pos.Subtract(e.scroll.Offset))
	require.True(t, caretHandleShown(e, c))
	e.TypedRune('x')
	assert.False(t, caretHandleShown(e, c), "typing a rune dismisses the caret handle")

	touchTap(e, e.caretRect(0, 2, true).Pos.Subtract(e.scroll.Offset))
	require.True(t, caretHandleShown(e, c))
	e.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	assert.False(t, caretHandleShown(e, c), "a key press dismisses the caret handle")
}

func TestEntry_CaretHandle_HidesOnFocusLoss(t *testing.T) {
	e, c := showHandlesEntry(t)

	touchTap(e, e.caretRect(0, 2, true).Pos.Subtract(e.scroll.Offset))
	require.True(t, caretHandleShown(e, c))

	c.Unfocus()
	assert.False(t, caretHandleShown(e, c), "focus loss dismisses the caret handle")

	// Re-focusing without a touch must not resurrect it.
	c.Focus(e)
	e.Refresh()
	assert.False(t, caretHandleShown(e, c), "only a touch tap re-arms the handle")
}

func TestEntry_CaretHandle_InactivityTimeout(t *testing.T) {
	e, c := showHandlesEntry(t)

	touchTap(e, e.caretRect(0, 2, true).Pos.Subtract(e.scroll.Offset))
	require.True(t, caretHandleShown(e, c))

	// The per-frame sync enforces the timeout; simulate its next tick after
	// the deadline instead of sleeping.
	e.caretHandle.shownAt = time.Now().Add(-caretHandleTimeout - time.Second)
	e.updateCaretHandle()
	assert.False(t, caretHandleShown(e, c), "the caret handle times out after inactivity")
	assert.False(t, e.caretHandle.active)
}

func TestEntry_CaretHandle_SelectionHandlesTakeOver(t *testing.T) {
	e, c := showHandlesEntry(t)

	touchTap(e, e.caretRect(0, 2, true).Pos.Subtract(e.scroll.Offset))
	require.True(t, caretHandleShown(e, c))

	touchSelect(e, 0, 0, 0, 5)
	assert.False(t, caretHandleShown(e, c), "a selection dismisses the caret handle")
	assert.Contains(t, c.Overlays().List(), e.selHandles, "the selection handles take over")
}

func TestEntry_CaretHandle_ShownForPassword(t *testing.T) {
	// Unlike the selection handles (word boundaries would reveal concealed
	// text), the caret handle carries no information and follows native
	// platforms in staying available on password fields.
	test.NewTempApp(t)
	e := NewPasswordEntry()
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(200, 60))
	e.SetText("secret words")
	w.Canvas().Focus(e)

	touchTap(e, e.caretRect(0, 3, true).Pos.Subtract(e.scroll.Offset))
	assert.True(t, caretHandleShown(e, w.Canvas()))
}
