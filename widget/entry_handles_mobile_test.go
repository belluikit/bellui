//go:build mobile

package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// The mobile selection-handle overlay (#758): shown while a focused entry has
// a non-empty touch-initiated selection (long-press, double-tap, touch drag,
// Select all from the context menu), hidden when the selection or focus goes
// away. Keyboard and programmatic selections summon no handles.

// touchSelect makes a touch-initiated selection, the way LongPressed or a
// touch drag would (tests select exact ranges, so they mark the flag the
// gesture paths set).
func touchSelect(e *Entry, startRow, startCol, endRow, endCol int) {
	e.Select(startRow, startCol, endRow, endCol)
	e.markTouchSelection()
}

func showHandlesEntry(t *testing.T) (*Entry, core.Canvas) {
	t.Helper()
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(200, 100))
	e.SetText("hello world this is a test\nsecond line of text here")
	w.Canvas().Focus(e)
	return e, w.Canvas()
}

func TestEntry_SelectionHandles_ShowHide(t *testing.T) {
	e, c := showHandlesEntry(t)
	require.Empty(t, c.Overlays().List())

	touchSelect(e, 0, 0, 0, 5)
	require.NotNil(t, e.selHandles)
	assert.Contains(t, c.Overlays().List(), e.selHandles, "handles overlay shown for a selection")
	assert.True(t, e.selHandles.start.Visible())
	assert.True(t, e.selHandles.end.Visible())
	assert.Less(t, e.selHandles.start.Position().X, e.selHandles.end.Position().X,
		"start handle sits under the selection start, end handle under its end")

	e.Unselect()
	assert.NotContains(t, c.Overlays().List(), e.selHandles, "handles hidden when the selection clears")

	touchSelect(e, 0, 0, 0, 5)
	require.Contains(t, c.Overlays().List(), e.selHandles)
	c.Unfocus()
	assert.NotContains(t, c.Overlays().List(), e.selHandles, "handles hidden on focus loss")
}

func TestEntry_SelectionHandles_HiddenForPassword(t *testing.T) {
	test.NewTempApp(t)
	e := NewPasswordEntry()
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(200, 60))
	e.SetText("secret words")
	w.Canvas().Focus(e)

	e.selectAll()
	e.markTouchSelection()
	if e.selHandles != nil {
		assert.NotContains(t, w.Canvas().Overlays().List(), e.selHandles,
			"a concealed field offers no draggable selection handles")
	}
}

func TestEntry_SelectionHandles_DragExtendsSelection(t *testing.T) {
	e, _ := showHandlesEntry(t)
	touchSelect(e, 0, 0, 0, 5) // "hello"
	require.NotNil(t, e.selHandles)
	require.Equal(t, "hello", e.SelectedText())

	// Drag the end handle one word to the right ("hello world").
	end := e.selHandles.end
	target := e.caretRect(0, 11, true) // content coords of the target edge
	fingerOverlay := target.Pos.Subtract(e.scroll.Offset).
		Add(hostAbsolutePosition(e).Subtract(e.selHandles.Position())).
		AddXY(0, target.Size.Height+end.Size().Height/2) // finger on the handle dot below the line
	end.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: fingerOverlay.Subtract(end.Position())}})
	end.DragEnd()

	assert.Equal(t, "hello world", e.SelectedText(), "dragging the end handle extends the selection")
	assert.Equal(t, 11, e.CursorColumn, "the caret follows the dragged edge")

	// The handles can never cross: dragging the end handle before the start
	// clamps to keep at least one rune selected.
	before := e.caretRect(0, 0, true)
	fingerOverlay = before.Pos.Subtract(e.scroll.Offset).
		Add(hostAbsolutePosition(e).Subtract(e.selHandles.Position())).
		AddXY(0, before.Size.Height+end.Size().Height/2)
	end.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: fingerOverlay.Subtract(end.Position())}})
	end.DragEnd()

	assert.Equal(t, "h", e.SelectedText(), "clamped to a non-empty selection")
}

func TestEntry_SelectionHandles_DragStartHandle(t *testing.T) {
	e, _ := showHandlesEntry(t)
	touchSelect(e, 0, 6, 0, 11) // "world"
	require.NotNil(t, e.selHandles)
	require.Equal(t, "world", e.SelectedText())

	start := e.selHandles.start
	target := e.caretRect(0, 0, true)
	fingerOverlay := target.Pos.Subtract(e.scroll.Offset).
		Add(hostAbsolutePosition(e).Subtract(e.selHandles.Position())).
		AddXY(0, target.Size.Height+start.Size().Height/2)
	start.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: fingerOverlay.Subtract(start.Position())}})
	start.DragEnd()

	assert.Equal(t, "hello world", e.SelectedText(), "dragging the start handle extends the selection backwards")
}

func TestEntry_LongPressed_SelectsWordAndShowsHandles(t *testing.T) {
	e, c := showHandlesEntry(t)

	pos := e.caretRect(0, 2, true).Pos.Subtract(e.scroll.Offset) // inside "hello"
	e.LongPressed(&event.PointEvent{Position: pos})

	assert.Equal(t, "hello", e.SelectedText(), "long-press selects the word under the finger")
	require.NotNil(t, e.selHandles)
	assert.Contains(t, c.Overlays().List(), e.selHandles)
}

func TestEntry_LongPressed_WhitespaceSnapsToNearestWord(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(300, 100))
	e.SetText("hi      countryside")
	w.Canvas().Focus(e)

	// Long-press inside the space run, nearer its right edge: the following
	// word is selected — never the sliver-thin space run itself (#958).
	pos := e.caretRect(0, 7, true).Pos.Subtract(e.scroll.Offset)
	e.LongPressed(&event.PointEvent{Position: pos})
	assert.Equal(t, "countryside", e.SelectedText(), "whitespace long-press snaps to the nearest word")

	// Nearer the left edge of the run the preceding word wins.
	pos = e.caretRect(0, 3, true).Pos.Subtract(e.scroll.Offset)
	e.LongPressed(&event.PointEvent{Position: pos})
	assert.Equal(t, "hi", e.SelectedText())
}

func TestEntry_SelectionHandles_MenuReshownAfterAdjust(t *testing.T) {
	e, c := showHandlesEntry(t)
	touchSelect(e, 0, 0, 0, 5)
	require.NotNil(t, e.selHandles)
	require.Nil(t, e.popUp, "no menu while the drag is being set up")

	// Adjust the selection with the end handle; releasing it re-summons the
	// host's context menu so the new selection can be acted on.
	end := e.selHandles.end
	target := e.caretRect(0, 11, true)
	fingerOverlay := target.Pos.Subtract(e.scroll.Offset).
		Add(hostAbsolutePosition(e).Subtract(e.selHandles.Position())).
		AddXY(0, target.Size.Height+end.Size().Height/2)
	end.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: fingerOverlay.Subtract(end.Position())}})
	assert.Nil(t, e.popUp, "no menu mid-drag")
	end.DragEnd()

	require.NotNil(t, e.popUp, "context menu reappears after the handle adjustment")
	assert.True(t, e.popUp.Visible())
	assert.NotEmpty(t, c.Overlays().List())

	// A plain tap on a handle (touch down + up, no drag) also offers the menu.
	e.popUp.Hide()
	e.popUp = nil
	end.TouchDown(&mobile.TouchEvent{})
	end.TouchUp(&mobile.TouchEvent{})
	assert.NotNil(t, e.popUp, "tapping a handle re-offers the menu")
}

func TestEntry_TouchDragSelect_OffersMenu(t *testing.T) {
	e, _ := showHandlesEntry(t)
	require.Nil(t, e.popUp)

	// A touch drag-select across "hello" (touch down, drag, release).
	e.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: e.caretRect(0, 0, true).Pos.Subtract(e.scroll.Offset)}})
	end := e.caretRect(0, 5, true).Pos.Subtract(e.scroll.Offset)
	e.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: end}, Dragged: geo.Delta{DX: end.X}})
	e.DragEnd()

	require.Equal(t, "hello", e.SelectedText(), "touch drag selects")
	assert.NotNil(t, e.popUp, "the context menu appears when a touch drag-select finishes")
}
