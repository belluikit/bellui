//go:build mobile

package widget

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// rotatedHandlesEntry shows a focused Entry turned a quarter turn about its
// centre by a transform on its parent.
func rotatedHandlesEntry(t *testing.T) (*Entry, core.Canvas) {
	t.Helper()
	test.NewTempApp(t)
	e := NewMultiLineEntry()
	box := newShiftBox(geo.NewRotationAround(math.Pi/2, 100, 50), e)
	w := test.NewTempWindow(t, &bellui.Container{Objects: []core.CanvasObject{box}})
	w.Resize(geo.NewSize(400, 400))
	box.Move(geo.NewPos(100, 150))
	box.Resize(geo.NewSize(200, 100))
	e.Resize(geo.NewSize(200, 100))
	e.SetText("hello world this is a test\nsecond line of text here")
	w.Canvas().Focus(e)
	return e, w.Canvas()
}

// handleFoot is where a handle placed under the caret at (row, col) hangs
// from, in canvas-absolute coordinates: the caret's foot, as the entry draws it.
func handleFoot(e *Entry, row, col int) geo.Position {
	r := e.caretRect(row, col, true)
	return hostPointAbsolute(e, r.Pos.Subtract(e.scroll.Offset).AddXY(0, r.Size.Height))
}

// TestEntry_SelectionHandles_FollowRotation pins #1008: the handles of a
// rotated Entry sit under the selection's ends as drawn, and dragging one
// along the turned text extends the selection. Before, the offsets inside the
// entry were added to its drawn origin unturned, so the handles sat off the
// text and a drag moved the edge along the wrong axis.
func TestEntry_SelectionHandles_FollowRotation(t *testing.T) {
	e, _ := rotatedHandlesEntry(t)
	touchSelect(e, 0, 0, 0, 5) // "hello"
	require.NotNil(t, e.selHandles)
	require.Equal(t, "hello", e.SelectedText())

	// The control: under a quarter turn the text runs down the screen, so the
	// selection's two ends differ in y, not in x. Unturned offsets would put
	// them side by side.
	startFoot, endFoot := handleFoot(e, 0, 0), handleFoot(e, 0, 5)
	require.InDelta(t, startFoot.X, endFoot.X, 0.5, "the text is not turned: the test cannot see the rotation")
	require.Greater(t, endFoot.Y-startFoot.Y, float32(10))

	overlay := e.selHandles.Position()
	for name, hd := range map[string]struct {
		h    *entrySelectionHandle
		foot geo.Position
	}{"start": {e.selHandles.start, startFoot}, "end": {e.selHandles.end, endFoot}} {
		got := overlay.Add(hd.h.Position()).AddXY(hd.h.Size().Width/2, 0)
		assert.InDelta(t, hd.foot.X, got.X, 0.5, "%s handle x", name)
		assert.InDelta(t, hd.foot.Y, got.Y, 0.5, "%s handle y", name)
	}

	// Drag the end handle to under "world"'s end: the finger holds the dot,
	// a quarter of the dot below where it hangs, upright on the canvas.
	end := e.selHandles.end
	grab := end.Size().Height / 4
	end.grabY = grab
	finger := handleFoot(e, 0, 11).AddXY(0, grab)
	end.Dragged(&event.DragEvent{PointEvent: event.PointEvent{AbsolutePosition: finger}})
	end.DragEnd()
	assert.Equal(t, "hello world", e.SelectedText(), "dragging the end handle along the turned text extends the selection")
}

// TestEntry_CaretHandle_FollowsRotation pins #1008 for the caret handle and
// its drag.
func TestEntry_CaretHandle_FollowsRotation(t *testing.T) {
	e, _ := rotatedHandlesEntry(t)
	e.CursorRow, e.CursorColumn = 0, 5
	e.showCaretHandle()
	require.NotNil(t, e.caretHandle)
	require.True(t, e.caretHandle.grab.Visible())

	foot := handleFoot(e, 0, 5)
	grab := e.caretHandle.grab
	got := e.caretHandle.Position().Add(grab.Position()).AddXY(grab.Size().Width/2, 0)
	assert.InDelta(t, foot.X, got.X, 0.5, "caret handle x")
	assert.InDelta(t, foot.Y, got.Y, 0.5, "caret handle y")

	off := grab.Size().Height / 4
	grab.grabY = off
	grab.Dragged(&event.DragEvent{PointEvent: event.PointEvent{AbsolutePosition: handleFoot(e, 0, 11).AddXY(0, off)}})
	grab.DragEnd()
	assert.Equal(t, 11, e.CursorColumn, "dragging the caret handle along the turned text moves the caret")
}

// TestEntry_Loupe_FollowsRotation pins #1008 for the loupe: its caret marker
// sits over the drag target's top as the rotated entry draws it.
func TestEntry_Loupe_FollowsRotation(t *testing.T) {
	e, _ := rotatedHandlesEntry(t)
	touchSelect(e, 0, 0, 0, 5)
	require.NotNil(t, e.selHandles)
	end := e.selHandles.end
	grab := end.Size().Height / 4
	end.grabY = grab
	end.Dragged(&event.DragEvent{PointEvent: event.PointEvent{AbsolutePosition: handleFoot(e, 0, 11).AddXY(0, grab)}})
	l := e.selHandles.loupe
	require.True(t, l.Visible(), "loupe shown while the handle drag is active")

	top := hostPointAbsolute(e, e.caretRect(0, 11, true).Pos.Subtract(e.scroll.Offset)).Subtract(e.selHandles.Position())
	marker := l.Position().X + l.caret.Position().X + l.caret.Size().Width/2
	assert.InDelta(t, top.X, marker, 0.5, "the loupe's caret marker rides over the turned caret")
	end.DragEnd()
}

// TestRichEntry_CaretHandle_FollowsRotation pins #1008 for RichEntry's caret
// handle and its drag.
func TestRichEntry_CaretHandle_FollowsRotation(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: RichTextStyleInline, Text: "hello world this is a test\nsecond line of text here"},
	})
	box := newShiftBox(geo.NewRotationAround(math.Pi/2, 125, 60), e)
	w := test.NewTempWindow(t, &bellui.Container{Objects: []core.CanvasObject{box}})
	w.Resize(geo.NewSize(400, 400))
	box.Move(geo.NewPos(80, 140))
	box.Resize(geo.NewSize(250, 120))
	e.Resize(geo.NewSize(250, 120))
	w.Canvas().Focus(e)

	foot := func(row, col int) geo.Position {
		pos, size := e.caretRectAt(row, col, false)
		return hostPointAbsolute(e.super(), pos.AddXY(0, size.Height))
	}
	touchTapRich(e, caretPos(e, 0, 2))
	require.NotNil(t, e.caretHandle)
	grab := e.caretHandle.grab
	require.True(t, grab.Visible())
	got := e.caretHandle.Position().Add(grab.Position()).AddXY(grab.Size().Width/2, 0)
	want := foot(0, 2)
	assert.InDelta(t, want.X, got.X, 0.5, "caret handle x")
	assert.InDelta(t, want.Y, got.Y, 0.5, "caret handle y")

	off := grab.Size().Height / 4
	grab.grabY = off
	grab.Dragged(&event.DragEvent{PointEvent: event.PointEvent{AbsolutePosition: foot(0, 11).AddXY(0, off)}})
	grab.DragEnd()
	row, col := e.doc.offsetToRowCol(e.caret)
	assert.Equal(t, [2]int{0, 11}, [2]int{row, col}, "dragging the caret handle along the turned text moves the caret")
}
