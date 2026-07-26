package widget

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// distinctFont returns a font resource whose metrics differ from the theme's
// default proportional text font, so a segment rendered/measured with it is
// visibly and dimensionally distinguishable from the default.
func distinctFont() bellui.Resource {
	return test.Theme().Font(bellui.TextStyle{Monospace: true})
}

func absF(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// TestRichText_FontSource_UpdatePropagates verifies TextSegment.Update and
// Visual set the custom font on the produced canvas.Text so the render path
// honours it.
func TestRichText_FontSource_UpdatePropagates(t *testing.T) {
	src := distinctFont()
	seg := &TextSegment{Text: "Hello", Style: RichTextStyle{FontSource: src}}

	obj := seg.Visual().(*canvas.Text)
	assert.Equal(t, src, obj.FontSource)

	// Update must also apply it to an existing visual (the reuse path).
	existing := canvas.NewText("", nil)
	seg.Update(existing)
	assert.Equal(t, src, existing.FontSource)

	// A nil-source segment leaves FontSource nil (zero value, unchanged behaviour).
	plain := &TextSegment{Text: "Hello", Style: RichTextStyleInline}
	assert.Nil(t, plain.Visual().(*canvas.Text).FontSource)
}

// TestRichText_FontSource_AffectsMeasurement proves the measurement/layout path
// is source-aware: the same text measured with a distinctive font yields a
// different natural width than the theme font. If measurement ignored the
// source, layout would silently disagree with what the render path draws.
func TestRichText_FontSource_AffectsMeasurement(t *testing.T) {
	test.NewTempApp(t)

	plain := NewRichText(&TextSegment{Text: "Measure me", Style: RichTextStyleInline})
	custom := NewRichText(&TextSegment{Text: "Measure me", Style: RichTextStyle{
		Inline:     true,
		FontSource: distinctFont(),
	}})

	assert.NotEqual(t, plain.MinSize().Width, custom.MinSize().Width,
		"custom font must change the measured width")

	// rowWidth (the alignment/selection width) must also reflect the source.
	plain.Resize(plain.MinSize())
	custom.Resize(custom.MinSize())
	assert.NotEqual(t, plain.rowWidth(0, plain.Segments[0].(*TextSegment).size()),
		custom.rowWidth(0, custom.Segments[0].(*TextSegment).size()))
}

// TestRichText_FontSource_NilIsByteIdentical guards the zero-value contract:
// an explicit nil FontSource lays out and renders exactly like a segment that
// never mentions the field.
func TestRichText_FontSource_NilIsByteIdentical(t *testing.T) {
	test.NewTempApp(t)

	withField := NewRichText(&TextSegment{Text: "Same", Style: RichTextStyle{
		Inline:     true,
		FontSource: nil,
	}})
	without := NewRichText(&TextSegment{Text: "Same", Style: RichTextStyleInline})

	assert.Equal(t, without.MinSize(), withField.MinSize())

	imgA := test.NewWindow(without).Canvas().Capture()
	imgB := test.NewWindow(withField).Canvas().Capture()
	assert.Equal(t, imgA.(*image.NRGBA).Pix, imgB.(*image.NRGBA).Pix)
}

// TestRichText_FontSource_RendersDifferently mirrors canvas.Text's FontSource
// test: swapping the source on the segment changes the rendered pixels.
func TestRichText_FontSource_RendersDifferently(t *testing.T) {
	test.NewTempApp(t)

	seg := &TextSegment{Text: "Rendered", Style: RichTextStyleInline}
	rich := NewRichText(seg)
	w := test.NewWindow(rich)
	w.Resize(rich.MinSize())

	imgDefault := w.Canvas().Capture()

	seg.Style.FontSource = distinctFont()
	rich.Refresh()
	imgCustom := w.Canvas().Capture()

	assert.NotEqual(t, imgDefault.(*image.NRGBA).Pix, imgCustom.(*image.NRGBA).Pix,
		"custom font must change the rendered pixels")
}

// TestRichText_FontSource_MixedInlineLine renders two inline segments on one
// visual line, one with a custom font, and checks they share a baseline and
// flow left-to-right — the measured row width must place the second segment
// exactly after the first's source-aware width. A golden image locks the visual.
func TestRichText_FontSource_MixedInlineLine(t *testing.T) {
	test.NewTempApp(t)

	rich := NewRichText(
		&TextSegment{Text: "Plain ", Style: RichTextStyleInline},
		&TextSegment{Text: "Custom", Style: RichTextStyle{Inline: true, FontSource: distinctFont()}},
	)
	rich.Resize(rich.MinSize())

	objs := richTextRenderTexts(rich)
	require.Len(t, objs, 2)
	assert.Nil(t, objs[0].FontSource)
	assert.Equal(t, distinctFont(), objs[1].FontSource)
	// One visual line: the two segments are baseline-aligned (so their top Y may
	// differ slightly with different font ascents) but never a full row apart.
	assert.Less(t, absF(objs[1].Position().Y-objs[0].Position().Y), objs[0].MinSize().Height,
		"both segments sit on one visual line")
	assert.Greater(t, objs[1].Position().X, objs[0].Position().X, "second segment flows after the first")

	test.AssertObjectRendersToImage(t, "richtext/fontsource_mixed.png", rich)
}

// TestRichText_FontSource_Wraps verifies wrapping stays consistent with the
// custom-font measurement (no panic, and rows fit within the wrap width).
func TestRichText_FontSource_Wraps(t *testing.T) {
	test.NewTempApp(t)

	rich := NewRichText(&TextSegment{
		Text:  "one two three four five six seven eight nine ten",
		Style: RichTextStyle{Inline: true, FontSource: distinctFont()},
	})
	rich.Wrapping = bellui.TextWrapWord
	rich.Resize(geo.NewSize(80, 200))
	assert.NotPanics(t, func() { rich.Refresh() })
	assert.Greater(t, len(rich.rowBounds), 1, "content should wrap into multiple rows")
}

// TestRichText_FontSource_Truncates checks ellipsis truncation with a custom
// font renders without panic.
func TestRichText_FontSource_Truncates(t *testing.T) {
	test.NewTempApp(t)

	rich := NewRichText(&TextSegment{
		Text:  "a long single line that will not fit and must be truncated",
		Style: RichTextStyle{Inline: true, FontSource: distinctFont()},
	})
	rich.Truncation = bellui.TextTruncateEllipsis
	rich.Resize(geo.NewSize(80, 40))
	assert.NotPanics(t, func() { rich.Refresh() })
}

// TestRichText_FontSource_ComposesWithBlockquote guards that a per-segment
// FontSource composes with the blockquote preset instead of defeating the
// whole-struct identity check that drives blockquote indentation.
func TestRichText_FontSource_ComposesWithBlockquote(t *testing.T) {
	assert.True(t, RichTextStyleBlockquote.isBlockquote(), "the preset itself is a blockquote")

	quoteWithFont := RichTextStyleBlockquote
	quoteWithFont.FontSource = distinctFont()
	assert.True(t, quoteWithFont.isBlockquote(),
		"a blockquote carrying a custom font must still lay out as a blockquote")

	assert.False(t, RichTextStyleParagraph.isBlockquote(), "paragraph is not a blockquote")

	paraWithFont := RichTextStyleParagraph
	paraWithFont.FontSource = distinctFont()
	assert.False(t, paraWithFont.isBlockquote(), "a font source alone does not make a blockquote")
}
