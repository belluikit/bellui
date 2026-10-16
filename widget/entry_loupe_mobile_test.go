//go:build mobile

package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// The drag magnifier (pre-rebrand tracker #942): while a finger drags a selection handle, a
// loupe bubble above the dragged line magnifies the text around the target
// caret position; it follows the drag and disappears on release.

// dragEndHandleTo drags the entry's end selection handle so the selection
// edge lands on (row, col), without releasing.
func dragEndHandleTo(e *Entry, row, col int) {
	hd := e.selHandles.end
	target := e.caretRect(row, col, true)
	fingerOverlay := target.Pos.Subtract(e.scroll.Offset).
		Add(hostAbsolutePosition(e).Subtract(e.selHandles.Position())).
		AddXY(0, target.Size.Height+hd.Size().Height/2)
	hd.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: fingerOverlay.Subtract(hd.Position())}})
}

// caretOverlayPos is the caret's top corner in overlay coordinates.
func caretOverlayPos(e *Entry, row, col int) geo.Position {
	return e.caretRect(row, col, true).Pos.Subtract(e.scroll.Offset).
		Add(hostAbsolutePosition(e).Subtract(e.selHandles.Position()))
}

func TestEntry_Loupe_ShownWhileHandleDragActive(t *testing.T) {
	e, _ := showHandlesEntry(t)
	touchSelect(e, 0, 0, 0, 5) // "hello"
	require.NotNil(t, e.selHandles)
	l := e.selHandles.loupe
	require.NotNil(t, l)
	assert.False(t, l.Visible(), "no loupe before a drag starts")

	dragEndHandleTo(e, 0, 11)
	assert.True(t, l.Visible(), "loupe shown while the handle drag is active")
	assert.Equal(t, "hello world this is a test", l.text.Text,
		"loupe magnifies the dragged row's text")

	// The caret marker sits over the drag target's true canvas x.
	caretX := caretOverlayPos(e, 0, 11).X
	markerCenter := l.Position().X + l.caret.Position().X + l.caret.Size().Width/2
	assert.InDelta(t, caretX, markerCenter, 0.01,
		"caret marker rides over the dragged selection edge")

	e.selHandles.end.DragEnd()
	assert.False(t, l.Visible(), "loupe dismissed when the drag ends")
}

func TestEntry_Loupe_FlipsBelowNearCanvasTop(t *testing.T) {
	e, _ := showHandlesEntry(t)
	touchSelect(e, 0, 0, 0, 5)
	require.NotNil(t, e.selHandles)

	dragEndHandleTo(e, 0, 11)
	l := e.selHandles.loupe
	require.True(t, l.Visible())

	caret := caretOverlayPos(e, 0, 11)
	lineBottom := caret.Y + e.caretRect(0, 11, true).Size.Height
	assert.Greater(t, l.Position().Y, lineBottom,
		"with no room above the first line the loupe flips below the handle")
}

func TestEntry_Loupe_PlacedAboveWhenRoomAllows(t *testing.T) {
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	cont := &bellui.Container{Objects: []core.CanvasObject{e}}
	w := test.NewTempWindow(t, cont)
	w.Resize(geo.NewSize(220, 320))
	e.Resize(geo.NewSize(200, 80))
	e.Move(geo.NewPos(10, 200))
	e.SetText("hello world this is a test\nsecond line of text here")
	w.Canvas().Focus(e)

	touchSelect(e, 0, 0, 0, 5)
	require.NotNil(t, e.selHandles)
	dragEndHandleTo(e, 0, 11)
	l := e.selHandles.loupe
	require.True(t, l.Visible())

	caretTop := caretOverlayPos(e, 0, 11).Y
	assert.LessOrEqual(t, l.Position().Y+l.Size().Height, caretTop,
		"the loupe hovers above the dragged line when there is room")
}

func TestEntry_Loupe_HighlightSpansSelectedStretch(t *testing.T) {
	e, _ := showHandlesEntry(t)
	touchSelect(e, 0, 0, 0, 5)
	require.NotNil(t, e.selHandles)

	dragEndHandleTo(e, 0, 11)
	l := e.selHandles.loupe
	require.True(t, l.Visible())
	require.True(t, l.highlight.Visible(), "the selected stretch is highlighted")

	x1 := e.loupeRowVisualX(0, 0)
	x2 := e.loupeRowVisualX(0, 11)
	assert.InDelta(t, l.text.Position().X+x1*loupeZoom, l.highlight.Position().X, 0.01)
	assert.InDelta(t, (x2-x1)*loupeZoom, l.highlight.Size().Width, 0.01)
}

func TestEntry_Loupe_HiddenWithOverlay(t *testing.T) {
	e, c := showHandlesEntry(t)
	touchSelect(e, 0, 0, 0, 5)
	require.NotNil(t, e.selHandles)
	dragEndHandleTo(e, 0, 11)
	require.True(t, e.selHandles.loupe.Visible())

	c.Unfocus() // hides the whole handles overlay mid-drag
	assert.False(t, e.selHandles.loupe.Visible(),
		"loupe cannot outlive the handles overlay")
}

func TestEntry_Loupe_ShownDuringCaretHandleDrag(t *testing.T) {
	e, _ := showHandlesEntry(t)

	touchTap(e, e.caretRect(0, 2, true).Pos.Subtract(e.scroll.Offset))
	require.NotNil(t, e.caretHandle)
	l := e.caretHandle.loupe
	require.NotNil(t, l)
	assert.False(t, l.Visible(), "no loupe before the grab dot is dragged")

	// Drag the grab dot so the caret lands on column 11.
	target := e.caretRect(0, 11, true)
	fingerOverlay := target.Pos.Subtract(e.scroll.Offset).
		Add(hostAbsolutePosition(e).Subtract(e.caretHandle.Position())).
		AddXY(0, target.Size.Height+e.caretHandle.grab.Size().Height/2)
	e.caretHandle.grab.Dragged(&event.DragEvent{PointEvent: event.PointEvent{
		Position: fingerOverlay.Subtract(e.caretHandle.grab.Position()),
	}})

	assert.True(t, l.Visible(), "loupe shown while the caret drag is active")
	assert.Equal(t, "hello world this is a test", l.text.Text)
	assert.False(t, l.highlight.Visible(), "a caret drag highlights nothing")

	// The caret marker sits over the dragged caret's true canvas x.
	caret := e.caretRect(e.CursorRow, e.CursorColumn, true).Pos.Subtract(e.scroll.Offset).
		Add(hostAbsolutePosition(e).Subtract(e.caretHandle.Position()))
	markerCenter := l.Position().X + l.caret.Position().X + l.caret.Size().Width/2
	assert.InDelta(t, caret.X, markerCenter, 0.01)

	e.caretHandle.grab.DragEnd()
	assert.False(t, l.Visible(), "loupe dismissed when the caret drag ends")
}
