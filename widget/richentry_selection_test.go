package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// RichEntry must expose pointer/touch selection through the standard event
// interfaces so the desktop and mobile drivers drive it (pre-rebrand tracker #750).
var (
	_ event.Draggable      = (*RichEntry)(nil)
	_ event.DoubleTappable = (*RichEntry)(nil)
)

func laidOutRichEntry(t *testing.T, text string) (*RichEntry, bellui.Window) {
	t.Helper()
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: text}})
	w := test.NewWindow(e)
	w.Resize(geo.NewSize(400, 80))
	e.FocusGained()
	return e, w
}

func TestRichEntry_DoubleTapSelectsWord(t *testing.T) {
	test.NewTempApp(t)
	e, w := laidOutRichEntry(t, "Hello World")
	defer w.Close()

	e.DoubleTapped(&event.PointEvent{Position: geo.NewPos(10, 8)})
	assert.Equal(t, "Hello", e.SelectedText(), "double-tap selects the word under the pointer")
}

func TestRichEntry_DragSelectsRange(t *testing.T) {
	test.NewTempApp(t)
	e, w := laidOutRichEntry(t, "Hello World")
	defer w.Close()

	start := geo.NewPos(8, 8)
	end := geo.NewPos(380, 8) // past the end of the line → last column
	e.Dragged(&event.DragEvent{
		PointEvent: event.PointEvent{Position: end},
		Dragged:    geo.NewDelta(end.X-start.X, 0),
	})
	e.DragEnd()
	assert.Equal(t, "Hello World", e.SelectedText(), "a drag across the line selects the range")
}

func TestRichEntry_DragAcrossRows(t *testing.T) {
	test.NewTempApp(t)
	e, w := laidOutRichEntry(t, "ab\ncd")
	defer w.Close()

	// Anchor at the very start; drag well past the end of the second row.
	start := geo.NewPos(8, 6)
	end := geo.NewPos(380, 34)
	e.Dragged(&event.DragEvent{
		PointEvent: event.PointEvent{Position: end},
		Dragged:    geo.NewDelta(end.X-start.X, end.Y-start.Y),
	})
	e.DragEnd()
	assert.Equal(t, "ab\ncd", e.SelectedText(), "a drag spans multiple rows including the newline")
}

func TestRichEntry_DragEndCollapsesEmptySelection(t *testing.T) {
	test.NewTempApp(t)
	e, w := laidOutRichEntry(t, "Hello World")
	defer w.Close()

	pos := geo.NewPos(8, 8)
	e.Dragged(&event.DragEvent{PointEvent: event.PointEvent{Position: pos}, Dragged: geo.NewDelta(0, 0)})
	e.DragEnd()
	assert.False(t, e.selecting, "a zero-length drag leaves no selection")
	assert.Equal(t, "", e.SelectedText())
}

func TestRichEntry_TapClearsSelection(t *testing.T) {
	test.NewTempApp(t)
	e, w := laidOutRichEntry(t, "Hello World")
	defer w.Close()

	e.DoubleTapped(&event.PointEvent{Position: geo.NewPos(10, 8)})
	assert.NotEmpty(t, e.SelectedText())

	e.Tapped(&event.PointEvent{Position: geo.NewPos(10, 8)})
	assert.Equal(t, "", e.SelectedText(), "a single tap collapses the selection")
}
