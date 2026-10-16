//go:build mobile

package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/driver/mobile"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// Mobile touch selection for the read-only selectable surfaces (pre-rebrand tracker #944, the
// pre-rebrand tracker #758 follow-up): a hold-fired long-press on a selectable Label/RichText
// selects the word under the finger and summons the shared selection-handle
// overlay; dragging a handle adjusts the selection; keyboard/programmatic
// selections summon no handles.

func showHandlesLabel(t *testing.T) (*Label, core.Canvas) {
	t.Helper()
	test.NewTempApp(t)
	l := NewLabel("hello world this is a test\nsecond line of text here")
	l.Selectable = true
	l.Wrapping = bellui.TextWrapOff
	w := test.NewTempWindow(t, l)
	w.Resize(geo.NewSize(250, 100))
	return l, w.Canvas()
}

func TestSelectableLabel_LongPressed_SelectsWordAndShowsHandles(t *testing.T) {
	l, c := showHandlesLabel(t)
	require.Empty(t, c.Overlays().List())

	sel := &l.selection.selectable
	pos := sel.selectionHandleCaretRect(0, 2).Pos // inside "hello"
	l.selection.LongPressed(&event.PointEvent{Position: pos})

	assert.Equal(t, "hello", l.SelectedText(), "long-press selects the word under the finger")
	assert.True(t, sel.focussed, "long-press focuses the selectable so the highlight shows")
	require.NotNil(t, sel.selHandles)
	assert.Contains(t, c.Overlays().List(), sel.selHandles, "handles overlay shown for the selection")
	assert.True(t, sel.selHandles.start.Visible())
	assert.True(t, sel.selHandles.end.Visible())
	assert.Less(t, sel.selHandles.start.Position().X, sel.selHandles.end.Position().X,
		"start handle sits under the selection start, end handle under its end")
}

func TestSelectableLabel_LongPressed_WhitespaceSnapsToNearestWord(t *testing.T) {
	test.NewTempApp(t)
	l := NewLabel("hi      countryside")
	l.Selectable = true
	l.Wrapping = bellui.TextWrapOff
	w := test.NewTempWindow(t, l)
	w.Resize(geo.NewSize(300, 60))

	// Long-press inside the space run, nearer its right edge: the following
	// word is selected — never the space run itself (pre-rebrand tracker #958).
	sel := &l.selection.selectable
	l.selection.LongPressed(&event.PointEvent{Position: sel.selectionHandleCaretRect(0, 7).Pos})
	assert.Equal(t, "countryside", l.SelectedText(), "whitespace long-press snaps to the nearest word")
}

func TestSelectableLabel_SelectionHandles_HideOnClearAndUnfocus(t *testing.T) {
	l, c := showHandlesLabel(t)
	sel := &l.selection.selectable

	l.selection.LongPressed(&event.PointEvent{Position: sel.selectionHandleCaretRect(0, 2).Pos})
	require.NotNil(t, sel.selHandles)
	require.Contains(t, c.Overlays().List(), sel.selHandles)

	// A plain mobile tap clears the selection, which hides the handles.
	sel.Tapped(&event.PointEvent{Position: geo.NewPos(5, 5)})
	assert.NotContains(t, c.Overlays().List(), sel.selHandles, "handles hidden when the selection clears")
	assert.False(t, sel.touchSelection, "the touch-initiated flag resets with the selection")

	l.selection.LongPressed(&event.PointEvent{Position: sel.selectionHandleCaretRect(0, 2).Pos})
	require.Contains(t, c.Overlays().List(), sel.selHandles)
	c.Unfocus()
	assert.NotContains(t, c.Overlays().List(), sel.selHandles, "handles hidden on focus loss")
}

func TestSelectableLabel_SelectionHandles_DragExtendsSelection(t *testing.T) {
	l, _ := showHandlesLabel(t)
	sel := &l.selection.selectable

	l.selection.LongPressed(&event.PointEvent{Position: sel.selectionHandleCaretRect(0, 2).Pos})
	require.Equal(t, "hello", l.SelectedText())
	require.NotNil(t, sel.selHandles)

	// Drag the end handle one word to the right ("hello world").
	end := sel.selHandles.end
	target := sel.selectionHandleCaretRect(0, 11) // content coords of the target edge
	fingerOverlay := target.Pos.
		Add(hostAbsolutePosition(l.selection).Subtract(sel.selHandles.Position())).
		AddXY(0, target.Size.Height+end.Size().Height/2) // finger on the handle dot below the line
	end.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: fingerOverlay.Subtract(end.Position())}})
	end.DragEnd()

	assert.Equal(t, "hello world", l.SelectedText(), "dragging the end handle extends the selection")

	// The handles can never cross: dragging the end handle before the start
	// clamps to keep at least one rune selected.
	before := sel.selectionHandleCaretRect(0, 0)
	fingerOverlay = before.Pos.
		Add(hostAbsolutePosition(l.selection).Subtract(sel.selHandles.Position())).
		AddXY(0, before.Size.Height+end.Size().Height/2)
	end.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: fingerOverlay.Subtract(end.Position())}})
	end.DragEnd()

	assert.Equal(t, "h", l.SelectedText(), "clamped to a non-empty selection")
}

func TestSelectableLabel_SelectionHandles_DragStartHandle(t *testing.T) {
	l, _ := showHandlesLabel(t)
	sel := &l.selection.selectable

	l.selection.LongPressed(&event.PointEvent{Position: sel.selectionHandleCaretRect(0, 8).Pos}) // "world"
	require.Equal(t, "world", l.SelectedText())
	require.NotNil(t, sel.selHandles)

	start := sel.selHandles.start
	target := sel.selectionHandleCaretRect(0, 0)
	fingerOverlay := target.Pos.
		Add(hostAbsolutePosition(l.selection).Subtract(sel.selHandles.Position())).
		AddXY(0, target.Size.Height+start.Size().Height/2)
	start.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: fingerOverlay.Subtract(start.Position())}})
	start.DragEnd()

	assert.Equal(t, "hello world", l.SelectedText(), "dragging the start handle extends the selection backwards")
}

func TestSelectableLabel_ProgrammaticSelection_NoHandles(t *testing.T) {
	l, c := showHandlesLabel(t)
	sel := &l.selection.selectable

	sel.selectAll() // keyboard/programmatic path: no touch-initiated flag
	assert.NotEmpty(t, l.SelectedText())
	if sel.selHandles != nil {
		assert.NotContains(t, c.Overlays().List(), sel.selHandles,
			"a programmatic selection summons no handles")
	}
}

func TestSelectableLabel_TouchDragSelection_ShowsHandles(t *testing.T) {
	l, c := showHandlesLabel(t)
	sel := &l.selection.selectable

	// A touch drag-select is touch-initiated, so it gets handles too.
	sel.TouchDown(nil)
	from := sel.selectionHandleCaretRect(0, 0).Pos
	to := sel.selectionHandleCaretRect(0, 5).Pos
	sel.Dragged(&event.DragEvent{
		PointEvent: event.PointEvent{Position: to},
		Dragged:    geo.NewDelta(to.X-from.X, to.Y-from.Y),
	})
	sel.DragEnd()
	sel.TouchUp(nil)

	require.True(t, sel.selecting)
	require.NotNil(t, sel.selHandles)
	assert.Contains(t, c.Overlays().List(), sel.selHandles, "touch drag-select summons handles")
}

func TestSelectableRichText_LongPressed_SelectsWordAndShowsHandles(t *testing.T) {
	test.NewTempApp(t)
	rt := NewRichTextWithText("alpha beta gamma")
	rt.Selectable = true
	w := test.NewTempWindow(t, rt)
	w.Resize(geo.NewSize(250, 100))

	sel := &rt.selection.selectable
	pos := sel.selectionHandleCaretRect(0, 7).Pos // inside "beta"
	rt.selection.LongPressed(&event.PointEvent{Position: pos})

	assert.Equal(t, "beta", rt.SelectedText(), "long-press selects the word in a selectable RichText")
	require.NotNil(t, sel.selHandles)
	assert.Contains(t, w.Canvas().Overlays().List(), sel.selHandles)
}

func TestEntry_InnerSelectable_NeverOwnsHandles(t *testing.T) {
	e, c := showHandlesEntry(t)

	touchSelect(e, 0, 0, 0, 5)
	require.NotNil(t, e.selHandles)
	require.Contains(t, c.Overlays().List(), e.selHandles)

	// The Entry owns the overlay; its embedded selectable must never spawn a
	// second one of its own.
	assert.False(t, e.sel.ownsSelectionHandles())
	assert.Nil(t, e.sel.selHandles)
}

func TestSelectableLabel_SelectionHandles_MenuReshownAfterAdjust(t *testing.T) {
	l, c := showHandlesLabel(t)
	sel := &l.selection.selectable

	l.selection.LongPressed(&event.PointEvent{Position: sel.selectionHandleCaretRect(0, 2).Pos})
	require.NotNil(t, sel.selHandles)
	overlaysBefore := len(c.Overlays().List())

	end := sel.selHandles.end
	target := sel.selectionHandleCaretRect(0, 11)
	fingerOverlay := target.Pos.
		Add(hostAbsolutePosition(l.selection).Subtract(sel.selHandles.Position())).
		AddXY(0, target.Size.Height+end.Size().Height/2)
	end.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: fingerOverlay.Subtract(end.Position())}})
	end.DragEnd()

	assert.Greater(t, len(c.Overlays().List()), overlaysBefore,
		"the Copy menu reappears after the handle adjustment")
}

func TestSelectableLabel_TouchDragSelect_OffersMenu(t *testing.T) {
	l, c := showHandlesLabel(t)
	sel := &l.selection.selectable

	// A touch drag-select across "hello" (touch down, drag, release).
	sel.TouchDown(&mobile.TouchEvent{PointEvent: event.PointEvent{Position: sel.selectionHandleCaretRect(0, 0).Pos}})
	overlaysBefore := len(c.Overlays().List())
	end := sel.selectionHandleCaretRect(0, 5).Pos
	l.selection.Dragged(&event.DragEvent{
		PointEvent: event.PointEvent{Position: end},
		Dragged:    geo.Delta{DX: end.X},
	})
	l.selection.DragEnd()

	require.Equal(t, "hello", l.SelectedText(), "touch drag selects")
	assert.Greater(t, len(c.Overlays().List()), overlaysBefore,
		"the Copy menu appears when a touch drag-select finishes")
}
