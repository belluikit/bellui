package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// TestRichText_IntrinsicSize_WordWrap verifies IntrinsicSize reports the natural
// (unwrapped) extent of a word-wrapping RichText — even after it has been laid
// out narrow enough to actually wrap — and leaves the widget's wrapping state
// unchanged. A plain MinSize collapses width to ~padding when wrapping.
func TestRichText_IntrinsicSize_WordWrap(t *testing.T) {
	test.NewTempApp(t)
	const content = "the quick brown fox jumps over the lazy dog"

	// Reference: an unwrapped widget reports its natural size via MinSize.
	ref := NewRichTextWithText(content)
	ref.Wrapping = bellui.TextWrapOff
	want := ref.MinSize()

	// A word-wrapping widget, realized and resized narrow so it actually wraps.
	rt := NewRichTextWithText(content)
	rt.Wrapping = bellui.TextWrapWord
	test.TempWidgetRenderer(t, rt) // build the renderer (live, rendered state)
	rt.Resize(geo.NewSize(40, 200))

	// Sanity: wrapping collapses the reported min width well below natural.
	assert.Less(t, rt.MinSize().Width, want.Width,
		"a word-wrapping RichText should report ~0 min width")

	// IntrinsicSize recovers the natural unwrapped size.
	got := rt.IntrinsicSize()
	assert.Equal(t, want.Width, got.Width, "intrinsic width should equal the unwrapped natural width")
	assert.Equal(t, want.Height, got.Height, "intrinsic height should equal the unwrapped natural height")

	// The call must not change the widget: wrapping is restored and the wrapped
	// min width is still collapsed.
	assert.Equal(t, bellui.TextWrapWord, rt.Wrapping, "Wrapping must be restored")
	assert.Less(t, rt.MinSize().Width, want.Width, "wrapped MinSize must be unchanged after IntrinsicSize")
}

// TestRichText_IntrinsicSize_WrapOffMatchesMinSize verifies that with wrapping
// off, IntrinsicSize is exactly MinSize (the no-op fast path).
func TestRichText_IntrinsicSize_WrapOffMatchesMinSize(t *testing.T) {
	test.NewTempApp(t)
	rt := NewRichTextWithText("hello world\nsecond line")
	rt.Wrapping = bellui.TextWrapOff
	assert.Equal(t, rt.MinSize(), rt.IntrinsicSize())
}

// TestRichText_NaturalSize_MatchesRefresh pins the side-effect-free naturalSize
// path to the double-Refresh oracle (intrinsicSizeViaRefresh) across a matrix of
// single text-segment content, wrapping modes and styles. They must be pixel-
// identical, and naturalSize must report ok for this content.
func TestRichText_NaturalSize_MatchesRefresh(t *testing.T) {
	test.NewTempApp(t)
	texts := []string{
		"hello",
		"hello world the quick brown fox",
		"line one\nsecond longer line\nthird",
		"trailing newline\n",
		"\nleading newline",
		"a\n\nb", // blank middle line
		"",
		"oneword",
		"日本語のテキスト\n二行目",
	}
	wraps := []bellui.TextWrap{bellui.TextWrapWord, bellui.TextWrapBreak}
	styles := []bellui.TextStyle{{}, {Bold: true}, {Italic: true}, {Monospace: true}}

	for _, txt := range texts {
		for _, wrap := range wraps {
			for _, style := range styles {
				rt := NewRichTextWithText(txt)
				rt.Wrapping = wrap
				rt.Segments[0].(*TextSegment).Style.TextStyle = style
				test.TempWidgetRenderer(t, rt)
				rt.Resize(geo.NewSize(30, 200)) // force actual wrapping

				got, ok := rt.naturalSize()
				assert.True(t, ok, "naturalSize should handle single text-segment content (%q)", txt)
				want := rt.intrinsicSizeViaRefresh()
				assert.Equal(t, want, got,
					"naturalSize != viaRefresh for text=%q wrap=%v style=%+v", txt, wrap, style)
			}
		}
	}
}

// TestRichText_NaturalSize_FallsBack verifies naturalSize declines content it
// cannot measure cheaply (multi-segment, scrolling) so IntrinsicSize routes to
// the refresh fallback.
func TestRichText_NaturalSize_FallsBack(t *testing.T) {
	test.NewTempApp(t)

	multi := NewRichText(&TextSegment{Text: "a", Style: RichTextStyleInline}, &TextSegment{Text: "b", Style: RichTextStyleInline})
	multi.Wrapping = bellui.TextWrapWord
	if _, ok := multi.naturalSize(); ok {
		t.Error("naturalSize should decline multi-segment content")
	}

	// Active folds hide lines, changing the extent, so naturalSize declines and the
	// fold-aware refresh path measures it instead.
	folded := NewRichTextWithText("line0\nline1\nline2\nline3")
	folded.Wrapping = bellui.TextWrapWord
	folded.setHiddenRanges([]LineRange{{Start: 1, End: 2}})
	if _, ok := folded.naturalSize(); ok {
		t.Error("naturalSize should decline folded content")
	}
}

// TestLabel_IntrinsicSize verifies Label.IntrinsicSize reports the natural
// unwrapped extent (matching an unwrapped reference Label) even when the label
// is wrapping and laid out narrow, and leaves the wrapping state unchanged.
func TestLabel_IntrinsicSize(t *testing.T) {
	test.NewTempApp(t)
	const content = "the quick brown fox jumps over the lazy dog"

	ref := NewLabel(content)
	ref.Wrapping = bellui.TextWrapOff
	want := ref.MinSize()

	l := NewLabel(content)
	l.Wrapping = bellui.TextWrapWord
	test.TempWidgetRenderer(t, l)
	l.Resize(geo.NewSize(40, 200)) // force wrapping

	assert.Less(t, l.MinSize().Width, want.Width, "a wrapping label should report ~0 min width")

	got := l.IntrinsicSize()
	assert.Equal(t, want, got, "Label.IntrinsicSize should equal the unwrapped natural size")
	assert.Equal(t, bellui.TextWrapWord, l.Wrapping, "Wrapping must be unchanged")
	assert.Less(t, l.MinSize().Width, want.Width, "wrapped MinSize must be unchanged after IntrinsicSize")
}

// TestLabel_IntrinsicSize_BeforeRender verifies IntrinsicSize works before the
// label has ever been rendered (the RichText provider is created lazily).
func TestLabel_IntrinsicSize_BeforeRender(t *testing.T) {
	test.NewTempApp(t)
	l := NewLabel("never rendered")
	l.Wrapping = bellui.TextWrapWord

	ref := NewLabel("never rendered")
	ref.Wrapping = bellui.TextWrapOff
	assert.Equal(t, ref.MinSize(), l.IntrinsicSize())
}

// TestEntry_IntrinsicSize verifies Entry.IntrinsicSize reports the natural extent
// of multi-line content as if wrapping were off.
func TestEntry_IntrinsicSize(t *testing.T) {
	test.NewTempApp(t)
	const content = "short\na considerably longer second line of text\nmid"

	e := NewMultiLineEntry()
	e.Wrapping = bellui.TextWrapWord
	e.SetText(content)
	test.TempWidgetRenderer(t, e)
	e.Resize(geo.NewSize(40, 200)) // force wrapping (and the entry's own scroller)

	got := e.IntrinsicSize()
	// The widest logical line drives the width — far beyond the wrapped/scrolled
	// MinSize floor — and the height spans the three lines. (A scrolled provider's
	// MinSize collapses to the scroll minimum, so IntrinsicSize must report the
	// content extent instead.)
	wide := bellui.MeasureText("a considerably longer second line of text", theme.TextSize(), bellui.TextStyle{}).Width
	assert.GreaterOrEqual(t, got.Width, wide, "intrinsic width should cover the widest line")
	assert.Less(t, got.Width, wide+40, "intrinsic width should be the widest line plus modest padding")
	assert.Greater(t, got.Width, e.MinSize().Width, "intrinsic width should exceed the scrolled min width")
	assert.Greater(t, got.Height, 3*bellui.MeasureText("x", theme.TextSize(), bellui.TextStyle{}).Height,
		"intrinsic height should span the three lines")
}

// TestRichText_IntrinsicSize_EllipsisRoundTrip is the #1045 repro: a truncating
// (ellipsis) RichText resized to exactly its own IntrinsicSize().Width must render
// the full text, not a truncated "…" form. The bug was that the available width was
// floored to an int before wrapping, while the measured intrinsic width kept its
// fractional part, so a fractionally-wide text never fit its own floored budget.
func TestRichText_IntrinsicSize_EllipsisRoundTrip(t *testing.T) {
	test.NewTempApp(t)
	const content = "Ready"

	rt := NewRichTextWithText(content)
	rt.Truncation = bellui.TextTruncateEllipsis
	test.TempWidgetRenderer(t, rt)

	intrinsic := rt.IntrinsicSize()
	rt.Resize(geo.NewSize(intrinsic.Width, intrinsic.Height))

	texts := richTextRenderTexts(rt)
	assert.Len(t, texts, 1)
	assert.Equal(t, content, texts[0].Text,
		"text sized to its own intrinsic width must not be truncated")
}

// TestLabel_IntrinsicSize_EllipsisRoundTrip is the #1045 repro at the Label level:
// the documented pattern of sizing a container/tooltip to a truncating label's
// IntrinsicSize must show the full text.
func TestLabel_IntrinsicSize_EllipsisRoundTrip(t *testing.T) {
	test.NewTempApp(t)
	const content = "Ready"

	l := NewLabel(content)
	l.Truncation = bellui.TextTruncateEllipsis
	test.TempWidgetRenderer(t, l)

	intrinsic := l.IntrinsicSize()
	l.Resize(geo.NewSize(intrinsic.Width, intrinsic.Height))

	assert.False(t, l.isTruncated(), "label sized to its intrinsic width must not be truncated")

	texts := richTextRenderTexts(l.provider)
	assert.Len(t, texts, 1)
	assert.Equal(t, content, texts[0].Text,
		"label text sized to its own intrinsic width must not show an ellipsis")
}
