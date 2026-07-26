package widget

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/stretchr/testify/assert"
)

func boldStyle() RichTextStyle {
	return RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Bold: true}}
}

func normalStyle() RichTextStyle {
	return RichTextStyle{Inline: true}
}

func segTexts(segs []RichTextSegment) []string {
	out := make([]string, 0, len(segs))
	for _, s := range segs {
		out = append(out, s.(*TextSegment).Text)
	}
	return out
}

// --- document model ---

func TestRichEntryDoc_InsertSplitsOnStyle(t *testing.T) {
	d := newRichEntryDoc()
	d.setSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "Hello World"}})

	// Insert bold "X" in the middle: splits into three runs.
	pos := d.insertText(6, []rune("X"), boldStyle())
	assert.Equal(t, 7, pos)
	assert.Equal(t, []string{"Hello ", "X", "World"}, segTexts(d.segments()))
	assert.Equal(t, boldStyle(), d.segments()[1].(*TextSegment).Style)
	assert.Equal(t, "Hello XWorld", d.String())
}

func TestRichEntryDoc_InsertSameStyleGrowsRun(t *testing.T) {
	d := newRichEntryDoc()
	d.setSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "Helo"}})

	d.insertText(3, []rune("l"), normalStyle())
	assert.Equal(t, []string{"Hello"}, segTexts(d.segments()))
	assert.Equal(t, "Hello", d.String())
}

func TestRichEntryDoc_DeleteMergesAdjacentSameStyle(t *testing.T) {
	d := newRichEntryDoc()
	d.setSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "Hello "},
		&TextSegment{Style: boldStyle(), Text: "X"},
		&TextSegment{Style: normalStyle(), Text: "World"},
	})
	// Delete the bold "X": the two normal runs become adjacent and merge.
	removed := d.deleteRange(6, 7)
	assert.Equal(t, "X", string(removed))
	assert.Equal(t, []string{"Hello World"}, segTexts(d.segments()))
}

func TestRichEntryDoc_TextBetweenAndLen(t *testing.T) {
	d := newRichEntryDoc()
	d.setSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "Hello "},
		&TextSegment{Style: boldStyle(), Text: "World"},
	})
	assert.Equal(t, 11, d.len())
	assert.Equal(t, "lo Wo", d.textBetween(3, 8))
	assert.Equal(t, "", d.textBetween(5, 5))
}

func TestRichEntryDoc_ApplyStyleSplitsRange(t *testing.T) {
	d := newRichEntryDoc()
	d.setSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "Hello World"}})
	d.applyStyle(6, 11, boldStyle()) // bold "World"
	assert.Equal(t, []string{"Hello ", "World"}, segTexts(d.segments()))
	assert.Equal(t, boldStyle(), d.segments()[1].(*TextSegment).Style)
}

func TestRichEntryDoc_RowColMappingRoundTrip(t *testing.T) {
	d := newRichEntryDoc()
	d.setSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "ab"},
		&TextSegment{Style: boldStyle(), Text: "cd\nef"},
	})
	// "abcd\nef": offsets 0..7, newline at index 4.
	for _, off := range []int{0, 2, 4, 5, 7} {
		row, col := d.offsetToRowCol(off)
		assert.Equal(t, off, d.rowColToOffset(row, col), "offset %d -> (%d,%d)", off, row, col)
	}
	r, c := d.offsetToRowCol(6) // "ab cd\ne|f"
	assert.Equal(t, 1, r)
	assert.Equal(t, 1, c)
}

// --- widget behaviour ---

func newTestRichEntry(t *testing.T) (*RichEntry, bellui.Window) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "Hello "},
		&TextSegment{Style: boldStyle(), Text: "World"},
	})
	w := test.NewWindow(e)
	w.Resize(geo.NewSize(200, 80))
	return e, w
}

func TestRichEntry_TypeInheritsCaretStyle(t *testing.T) {
	e, w := newTestRichEntry(t)
	defer w.Close()

	e.FocusGained()
	e.caret = 8 // inside the bold "World" ("Hello Wo|rld")
	e.anchor = 8
	e.TypedRune('Z')
	assert.Equal(t, "Hello WoZrld", e.String())
	// Still two runs — the Z joined the bold run rather than splitting it.
	assert.Equal(t, []string{"Hello ", "WoZrld"}, segTexts(e.Segments()))
	assert.Equal(t, 9, e.caret)
}

func TestRichEntry_BackspaceAcrossBoundary(t *testing.T) {
	e, w := newTestRichEntry(t)
	defer w.Close()

	e.caret, e.anchor = 6, 6 // between "Hello " and "World"
	e.TypedKey(&event.KeyEvent{Name: event.KeyBackspace})
	assert.Equal(t, "HelloWorld", e.String())
	assert.Equal(t, 5, e.caret)
}

func TestRichEntry_ShiftSelectionAndCopy(t *testing.T) {
	e, w := newTestRichEntry(t)
	defer w.Close()

	e.FocusGained()
	e.caret, e.anchor = 0, 0
	e.KeyDown(&event.KeyEvent{Name: desktop.KeyShiftLeft})
	for i := 0; i < 5; i++ {
		e.TypedKey(&event.KeyEvent{Name: event.KeyRight})
	}
	from, to, ok := e.selection()
	assert.True(t, ok)
	assert.Equal(t, 0, from)
	assert.Equal(t, 5, to)
	assert.Equal(t, "Hello", e.SelectedText())
}

func TestRichEntry_ApplyStyleToSelection(t *testing.T) {
	e, w := newTestRichEntry(t)
	defer w.Close()

	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "Hello World"}})
	e.selecting, e.anchor, e.caret = true, 0, 5 // "Hello"
	e.ApplyStyle(boldStyle())
	assert.Equal(t, []string{"Hello", " World"}, segTexts(e.Segments()))
	assert.Equal(t, boldStyle(), e.Segments()[0].(*TextSegment).Style)
}

// TestRichEntry_CaretCrossesStyleBoundary proves the multi-segment geometry:
// the caret x inside the bold "World" run is past the width of the entire
// preceding "Hello " normal run, and monotonically increases across the boundary.
func TestRichEntry_CaretCrossesStyleBoundary(t *testing.T) {
	e, w := newTestRichEntry(t)
	defer w.Close()

	xAt := func(off int) float32 {
		e.caret = off
		pos, _ := e.caretRect()
		return pos.X
	}
	x0, x6, x9 := xAt(0), xAt(6), xAt(9)
	assert.Greater(t, x6, x0, "caret at the style boundary is past the start")
	assert.Greater(t, x9, x6, "caret inside the bold run is past the boundary")
}

func TestRichEntry_Render(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "Hello "},
		&TextSegment{Style: boldStyle(), Text: "World"},
	})
	e.Resize(geo.NewSize(150, 40))
	test.AssertObjectRendersToMarkup(t, "richentry/rich.xml", e)
}

func TestRichEntry_RenderFocusedSelection(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "Hello "},
		&TextSegment{Style: boldStyle(), Text: "World"},
	})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(200, 60))
	e.FocusGained()
	e.selecting, e.anchor, e.caret = true, 0, 5
	e.Refresh()
	test.AssertObjectRendersToMarkup(t, "richentry/rich_selected.xml", e)
}

func TestRichEntry_TypeOverSelectionUsesInsertPointStyle(t *testing.T) {
	e, w := newTestRichEntry(t) // "Hello "(normal) + "World"(bold)
	defer w.Close()

	// Select the bold "World" with the caret at the selection END (offset 11),
	// anchor at the start (6). Typing must inherit the style at the insertion
	// point (normal "Hello ") — not the bold run being deleted.
	e.selecting, e.anchor, e.caret = true, 6, 11
	e.TypedRune('!')
	assert.Equal(t, "Hello !", e.String())
	assert.Equal(t, []string{"Hello !"}, segTexts(e.Segments()))
	assert.Equal(t, normalStyle(), e.Segments()[0].(*TextSegment).Style)
}

func TestRichEntry_DeleteAtEndIsNoOp(t *testing.T) {
	e, w := newTestRichEntry(t)
	defer w.Close()

	changed := 0
	e.OnChanged = func() { changed++ }
	e.selecting, e.anchor, e.caret = false, e.doc.len(), e.doc.len()
	e.TypedKey(&event.KeyEvent{Name: event.KeyDelete})
	assert.Equal(t, "Hello World", e.String())
	assert.Equal(t, 0, changed, "Delete at end must not fire OnChanged")
}

func TestRichEntry_DownArrowOnLastRowGoesToEnd(t *testing.T) {
	e, w := newTestRichEntry(t)
	defer w.Close()

	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "abcdef\ngh"}})
	e.selecting, e.anchor, e.caret = false, 8, 8 // row 1, col 1 ("g|h")
	e.TypedKey(&event.KeyEvent{Name: event.KeyDown})
	assert.Equal(t, 9, e.caret, "Down on the last row moves to end of document")
}

func TestRichEntry_PasteNormalizesCRLF(t *testing.T) {
	e, w := newTestRichEntry(t)
	defer w.Close()

	e.selecting, e.anchor, e.caret = false, 0, 0
	cb := bellui.CurrentApp().Clipboard()
	cb.SetContent("a\r\nb\rc")
	e.pasteFromClipboard(cb)
	assert.Equal(t, "a\nb\ncHello World", e.String())
}

func TestRichEntryDoc_MergeStylesWithPointerWeights(t *testing.T) {
	d := newRichEntryDoc()
	bold1 := RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Weight: bellui.Ptr(bellui.WeightBold)}}
	bold2 := RichTextStyle{Inline: true, TextStyle: bellui.TextStyle{Weight: bellui.Ptr(bellui.WeightBold)}}
	// Two separately-constructed but value-equal bold styles must merge.
	d.setSegments([]RichTextSegment{
		&TextSegment{Style: bold1, Text: "foo"},
		&TextSegment{Style: bold2, Text: "bar"},
	})
	assert.Equal(t, []string{"foobar"}, segTexts(d.segments()))
}

func TestRichEntry_OnContentChanged(t *testing.T) {
	e, w := newTestRichEntry(t) // "Hello "(normal) + "World"(bold)
	defer w.Close()
	e.FocusGained()

	var last []RichTextSegment
	calls := 0
	e.OnContentChanged = func(segs []RichTextSegment) {
		calls++
		last = segs
	}

	e.caret, e.anchor = 6, 6
	e.TypedRune('X') // inserts normal-styled 'X' at the style boundary

	assert.Equal(t, 1, calls)
	assert.Equal(t, "Hello XWorld", segsPlain(last))
	// fires with the live document segments (same as Segments()).
	assert.Equal(t, segTexts(e.Segments()), segTexts(last))

	// A caret move (no content change) must not fire it.
	e.TypedKey(&event.KeyEvent{Name: event.KeyLeft})
	assert.Equal(t, 1, calls)
}

func segsPlain(segs []RichTextSegment) string {
	out := ""
	for _, s := range segs {
		out += s.Textual()
	}
	return out
}

func TestRichEntry_PasteInsertsPlainText(t *testing.T) {
	e, w := newTestRichEntry(t)
	defer w.Close()

	e.caret, e.anchor = 0, 0
	cb := bellui.CurrentApp().Clipboard()
	cb.SetContent("Hi ")
	e.pasteFromClipboard(cb)
	assert.Equal(t, "Hi Hello World", e.String())
}
