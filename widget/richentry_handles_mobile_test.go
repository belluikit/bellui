//go:build mobile

package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// Mobile touch text selection for RichEntry (#888, parity with Entry's #758
// and the selectable's #944): a hold-fired long-press selects the word under
// the finger and summons the shared selection-handle overlay; dragging a
// handle adjusts the selection with the drag magnifier over the target;
// keyboard/programmatic selections summon no handles.

func showHandlesRichEntry(t *testing.T) (*RichEntry, core.Canvas) {
	t.Helper()
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: RichTextStyleInline, Text: "hello world this is a test\nsecond line of text here"},
	})
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(250, 120))
	w.Canvas().Focus(e)
	return e, w.Canvas()
}

// longPressRichEntry long-presses the editor at (row, col) through the
// embedded selectable — the object the driver's hold-fire dispatch finds.
func longPressRichEntry(e *RichEntry, row, col int) {
	pos := e.selectionHandleCaretRect(row, col).Pos
	e.sel.LongPressed(&event.PointEvent{Position: pos})
}

func TestRichEntry_LongPressed_SelectsWordAndShowsHandles(t *testing.T) {
	e, c := showHandlesRichEntry(t)
	require.Empty(t, c.Overlays().List())

	longPressRichEntry(e, 0, 2) // inside "hello"

	assert.Equal(t, "hello", e.SelectedText(), "long-press selects the word under the finger")
	require.NotNil(t, e.selHandles)
	assert.Contains(t, c.Overlays().List(), e.selHandles, "handles overlay shown for the selection")
	assert.True(t, e.selHandles.start.Visible())
	assert.True(t, e.selHandles.end.Visible())
	assert.Less(t, e.selHandles.start.Position().X, e.selHandles.end.Position().X,
		"start handle sits under the selection start, end handle under its end")
}

func TestRichEntry_InnerSelectable_NeverOwnsHandles(t *testing.T) {
	e, c := showHandlesRichEntry(t)

	longPressRichEntry(e, 0, 2)
	require.NotNil(t, e.selHandles)
	require.Contains(t, c.Overlays().List(), e.selHandles)

	// The editor owns the overlay; its embedded selectable must never spawn a
	// second one of its own.
	assert.False(t, e.sel.ownsSelectionHandles())
	assert.Nil(t, e.sel.selHandles)
}

func TestRichEntry_SelectionHandles_HideOnClearAndUnfocus(t *testing.T) {
	e, c := showHandlesRichEntry(t)

	longPressRichEntry(e, 0, 2)
	require.NotNil(t, e.selHandles)
	require.Contains(t, c.Overlays().List(), e.selHandles)

	// A plain tap collapses the selection, which hides the handles.
	e.Tapped(&event.PointEvent{Position: e.selectionHandleCaretRect(1, 2).Pos})
	assert.NotContains(t, c.Overlays().List(), e.selHandles, "handles hidden when the selection clears")
	assert.False(t, e.touchSelection, "the touch-initiated flag resets with the selection")

	longPressRichEntry(e, 0, 2)
	require.Contains(t, c.Overlays().List(), e.selHandles)
	c.Unfocus()
	assert.NotContains(t, c.Overlays().List(), e.selHandles, "handles hidden on focus loss")
}

// dragRichEntryHandle drags one selection handle so its edge lands on
// (row, col), without releasing.
func dragRichEntryHandle(e *RichEntry, hd *entrySelectionHandle, row, col int) {
	target := e.selectionHandleCaretRect(row, col)
	fingerOverlay := target.Pos.
		Add(hostAbsolutePosition(e).Subtract(e.selHandles.Position())).
		AddXY(0, target.Size.Height+hd.Size().Height/2) // finger on the handle dot below the line
	hd.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: fingerOverlay.Subtract(hd.Position())}})
}

func TestRichEntry_SelectionHandles_DragExtendsSelection(t *testing.T) {
	e, _ := showHandlesRichEntry(t)

	longPressRichEntry(e, 0, 2)
	require.Equal(t, "hello", e.SelectedText())
	require.NotNil(t, e.selHandles)

	// Drag the end handle one word to the right ("hello world").
	dragRichEntryHandle(e, e.selHandles.end, 0, 11)
	e.selHandles.end.DragEnd()
	assert.Equal(t, "hello world", e.SelectedText(), "dragging the end handle extends the selection")

	// The handles can never cross: dragging the end handle before the start
	// clamps to keep at least one position selected.
	dragRichEntryHandle(e, e.selHandles.end, 0, 0)
	e.selHandles.end.DragEnd()
	assert.Equal(t, "h", e.SelectedText(), "clamped to a non-empty selection")
}

func TestRichEntry_SelectionHandles_DragStartHandle(t *testing.T) {
	e, _ := showHandlesRichEntry(t)

	longPressRichEntry(e, 0, 8) // "world"
	require.Equal(t, "world", e.SelectedText())
	require.NotNil(t, e.selHandles)

	dragRichEntryHandle(e, e.selHandles.start, 0, 0)
	e.selHandles.start.DragEnd()
	assert.Equal(t, "hello world", e.SelectedText(), "dragging the start handle extends the selection backwards")
}

func TestRichEntry_SelectionHandles_DragAcrossStyledRuns(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: RichTextStyleInline, Text: "plain "},
		&TextSegment{Style: RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Bold: true}}, Text: "bold"},
		&TextSegment{Style: RichTextStyleInline, Text: " tail"},
	})
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(250, 120))
	w.Canvas().Focus(e)

	longPressRichEntry(e, 0, 2) // "plain"
	require.Equal(t, "plain", e.SelectedText())
	require.NotNil(t, e.selHandles)

	dragRichEntryHandle(e, e.selHandles.end, 0, 10) // through the bold run
	e.selHandles.end.DragEnd()
	assert.Equal(t, "plain bold", e.SelectedText(), "handle drag crosses styled runs")
}

func TestRichEntry_ProgrammaticSelection_NoHandles(t *testing.T) {
	e, c := showHandlesRichEntry(t)

	e.selectAll() // keyboard/programmatic path: no touch-initiated flag
	assert.NotEmpty(t, e.SelectedText())
	if e.selHandles != nil {
		assert.NotContains(t, c.Overlays().List(), e.selHandles,
			"a programmatic selection summons no handles")
	}
}

func TestRichEntry_TouchDragSelection_ShowsHandles(t *testing.T) {
	e, c := showHandlesRichEntry(t)

	// A touch drag-select is touch-initiated, so it gets handles too. The
	// touch lands on the embedded selectable, which forwards the pointer
	// type and the drag to the editor.
	e.sel.TouchDown(&mobile.TouchEvent{})
	assert.True(t, e.touchPointer, "the selectable forwards the touch to the editor")
	from := e.selectionHandleCaretRect(0, 0).Pos
	to := e.selectionHandleCaretRect(0, 5).Pos
	e.sel.Dragged(&event.DragEvent{
		PointEvent: event.PointEvent{Position: to},
		Dragged:    geo.NewDelta(to.X-from.X, to.Y-from.Y),
	})
	e.sel.DragEnd()
	e.sel.TouchUp(&mobile.TouchEvent{})

	require.True(t, e.selecting)
	require.NotNil(t, e.selHandles)
	assert.Contains(t, c.Overlays().List(), e.selHandles, "touch drag-select summons handles")
}

func TestRichEntry_MouseDrag_NoHandles(t *testing.T) {
	e, c := showHandlesRichEntry(t)

	// A mouse drag-select (desktop pointer) summons no handles.
	from := e.selectionHandleCaretRect(0, 0).Pos
	to := e.selectionHandleCaretRect(0, 5).Pos
	e.Dragged(&event.DragEvent{
		PointEvent: event.PointEvent{Position: to},
		Dragged:    geo.NewDelta(to.X-from.X, to.Y-from.Y),
	})
	e.DragEnd()

	require.True(t, e.selecting)
	if e.selHandles != nil {
		assert.NotContains(t, c.Overlays().List(), e.selHandles,
			"a mouse drag-select summons no handles")
	}
}

func TestRichEntry_HandleAdjust_ReshowsMenu(t *testing.T) {
	e, c := showHandlesRichEntry(t)

	longPressRichEntry(e, 0, 2)
	require.NotNil(t, e.selHandles)
	overlaysBefore := len(c.Overlays().List())

	dragRichEntryHandle(e, e.selHandles.end, 0, 11)
	e.selHandles.end.DragEnd()

	assert.Greater(t, len(c.Overlays().List()), overlaysBefore,
		"the context menu reappears after the handle adjustment")
}

func TestRichEntry_Loupe_ShownWhileHandleDragActive(t *testing.T) {
	e, _ := showHandlesRichEntry(t)

	longPressRichEntry(e, 0, 2)
	require.NotNil(t, e.selHandles)
	l := e.selHandles.loupe
	require.NotNil(t, l, "RichEntry is a loupe host")
	assert.False(t, l.Visible(), "no loupe before a drag starts")

	dragRichEntryHandle(e, e.selHandles.end, 0, 11)
	assert.True(t, l.Visible(), "loupe shown while the handle drag is active")
	require.True(t, l.styled.Visible(), "the row copy renders as styled runs")
	assert.False(t, l.text.Visible(), "the plain single-style copy stays hidden")

	text := ""
	for _, run := range l.styled.Runs {
		text += run.Text
	}
	assert.Equal(t, "hello world this is a test", text, "loupe magnifies the dragged row's text")

	// The caret marker sits over the drag target's true canvas x.
	caretAbs := e.selectionHandleCaretRect(0, 11).Pos.
		Add(hostAbsolutePosition(e).Subtract(e.selHandles.Position()))
	markerCenter := l.Position().X + l.caret.Position().X + l.caret.Size().Width/2
	assert.InDelta(t, caretAbs.X, markerCenter, 0.01,
		"caret marker rides over the dragged selection edge")

	e.selHandles.end.DragEnd()
	assert.False(t, l.Visible(), "loupe dismissed when the drag ends")
}

func TestRichEntry_Loupe_StyledRunsKeepStyleAndZoom(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: RichTextStyleInline, Text: "plain "},
		&TextSegment{Style: RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Bold: true}}, Text: "bold"},
	})
	w := test.NewTempWindow(t, e)
	w.Resize(geo.NewSize(250, 120))
	w.Canvas().Focus(e)

	longPressRichEntry(e, 0, 2)
	require.NotNil(t, e.selHandles)
	dragRichEntryHandle(e, e.selHandles.end, 0, 10)
	l := e.selHandles.loupe
	require.True(t, l.Visible())
	require.True(t, l.styled.Visible())

	require.Len(t, l.styled.Runs, 2)
	assert.Equal(t, "plain ", l.styled.Runs[0].Text)
	assert.Equal(t, "bold", l.styled.Runs[1].Text)
	assert.True(t, l.styled.Runs[1].TextStyle.Bold, "the bold run keeps its style in the magnified copy")
	oneX := (&TextSegment{Style: RichTextStyleInline}).size()
	assert.InDelta(t, oneX*loupeZoom, l.styled.Runs[0].TextSize, 0.01,
		"run sizes are magnified by the loupe zoom")

	e.selHandles.end.DragEnd()
}

func TestSelectableRichText_Loupe_StyledRuns(t *testing.T) {
	test.NewTempApp(t)
	rt := NewRichText(
		&TextSegment{Style: RichTextStyleInline, Text: "alpha "},
		&TextSegment{Style: RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Bold: true}}, Text: "beta"},
		&TextSegment{Style: RichTextStyleInline, Text: " gamma"},
	)
	rt.Selectable = true
	w := test.NewTempWindow(t, rt)
	w.Resize(geo.NewSize(250, 100))

	sel := &rt.selection.selectable
	rt.selection.LongPressed(&event.PointEvent{Position: sel.selectionHandleCaretRect(0, 2).Pos})
	require.Equal(t, "alpha", rt.SelectedText())
	require.NotNil(t, sel.selHandles)
	l := sel.selHandles.loupe
	require.NotNil(t, l, "the selectable is a loupe host (#888 adoption)")

	end := sel.selHandles.end
	target := sel.selectionHandleCaretRect(0, 10)
	fingerOverlay := target.Pos.
		Add(hostAbsolutePosition(rt.selection).Subtract(sel.selHandles.Position())).
		AddXY(0, target.Size.Height+end.Size().Height/2)
	end.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: fingerOverlay.Subtract(end.Position())}})

	require.True(t, l.Visible(), "loupe shown for a selectable handle drag")
	require.True(t, l.styled.Visible())
	var runs []canvas.TextRun = l.styled.Runs
	require.Len(t, runs, 3)
	assert.Equal(t, "alpha ", runs[0].Text)
	assert.True(t, runs[1].TextStyle.Bold)

	end.DragEnd()
	assert.False(t, l.Visible(), "loupe dismissed when the drag ends")
}

func TestSelectableLabel_Loupe_ShownWhileHandleDragActive(t *testing.T) {
	l, _ := showHandlesLabel(t)
	sel := &l.selection.selectable

	l.selection.LongPressed(&event.PointEvent{Position: sel.selectionHandleCaretRect(0, 2).Pos})
	require.NotNil(t, sel.selHandles)
	lp := sel.selHandles.loupe
	require.NotNil(t, lp, "the selectable is a loupe host (#888 adoption)")
	assert.False(t, lp.Visible())

	end := sel.selHandles.end
	target := sel.selectionHandleCaretRect(0, 11)
	fingerOverlay := target.Pos.
		Add(hostAbsolutePosition(l.selection).Subtract(sel.selHandles.Position())).
		AddXY(0, target.Size.Height+end.Size().Height/2)
	end.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: fingerOverlay.Subtract(end.Position())}})

	require.True(t, lp.Visible(), "loupe shown while the handle drag is active")
	text := ""
	for _, run := range lp.styled.Runs {
		text += run.Text
	}
	assert.Equal(t, "hello world this is a test", text)

	end.DragEnd()
	assert.False(t, lp.Visible())
}
