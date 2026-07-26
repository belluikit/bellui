package widget

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// Tests for the bidi-aware inline-atom caret/selection geometry (#913): a row
// mixing right-to-left text with an inline image must place the caret and the
// selection highlight from the row's visual layout, not the LTR prefix walk.

// bidiInlineEntry builds a RichEntry with one row: RTL word, inline image, RTL
// word (document positions: 0..3 in the first word, 4 = the image, 5..8 in the
// second word).
func bidiInlineEntry(t *testing.T) (*RichEntry, *ImageSegment, bellui.Window) {
	t.Helper()
	img := &ImageSegment{Image: image.NewNRGBA(image.Rect(0, 0, 24, 16)), InlineImage: true}
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: RichTextStyle{Inline: true}, Text: "שלום"},
		img,
		&TextSegment{Style: RichTextStyle{Inline: true}, Text: "עולם"},
	})
	w := test.NewWindow(e)
	w.Resize(geo.NewSize(400, 80))
	return e, img, w
}

func TestRichEntry_BidiInlineImageDocColResolve(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	e, _, w := bidiInlineEntry(t)
	defer w.Close()
	p := &e.provider

	require.Equal(t, 1, p.rows())
	require.Equal(t, 9, e.doc.len())

	// Text columns skip the atom; the atom boundary resolves to box sides.
	tc, _, _, isBox := p.rowDocColResolve(0, 2)
	assert.False(t, isBox)
	assert.Equal(t, 2, tc)

	tc, box, after, isBox := p.rowDocColResolve(0, 4)
	assert.True(t, isBox)
	assert.False(t, after, "doc col 4 is the insertion point before the image")
	assert.Equal(t, 0, box)
	assert.Equal(t, 4, tc)

	_, box, after, isBox = p.rowDocColResolve(0, 5)
	assert.True(t, isBox)
	assert.True(t, after, "doc col 5 is the insertion point after the image")
	assert.Equal(t, 0, box)

	tc, _, _, isBox = p.rowDocColResolve(0, 7)
	assert.False(t, isBox)
	assert.Equal(t, 6, tc, "text columns after the atom shift down by one")

	assert.Equal(t, []int{4}, p.rowBoxDocPositions(0))
}

func TestRichEntry_BidiInlineImageCaret(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	e, img, w := bidiInlineEntry(t)
	defer w.Close()
	p := &e.provider

	th := e.Theme()
	innerPad := th.Size(theme.SizeNameInnerPadding)
	imgW := p.cachedSegmentVisual(img, 0).MinSize().Width
	require.Positive(t, imgW)

	x := func(docCol int) float32 {
		v, ok := p.bidiInlineColumnX(0, docCol, innerPad, true)
		require.True(t, ok, "bidi inline path must engage for doc col %d", docCol)
		return v
	}

	// Within the leading RTL word, advancing the logical position moves the
	// caret visually left — the LTR prefix walk would move it right.
	assert.Greater(t, x(0), x(1))
	assert.Greater(t, x(1), x(2))
	assert.Greater(t, x(2), x(3))

	// The atom boundary carets are the image's own edges: stepping over the
	// image advances by exactly its width.
	seg1 := e.doc.segments()[0].(*TextSegment)
	w1, _ := bellui.MeasureTextFull(seg1.Text, seg1.size(), seg1.Style.TextStyle, seg1.Style.FontSource)
	off := innerPad - p.inset.Width
	assert.InDelta(t, off+w1.Width, x(4), 0.6, "caret before the image sits at its edge")
	assert.InDelta(t, imgW, x(5)-x(4), 0.6, "stepping over the image advances by its width")

	// caretRect routes the atom row through the bidi path.
	e.moveCaret(4, false)
	pos4, _ := e.caretRect()
	e.moveCaret(5, false)
	pos5, _ := e.caretRect()
	assert.InDelta(t, imgW, pos5.X-pos4.X, 0.6, "caretRect steps over the image by its width")
	e.moveCaret(0, false)
	pos0, _ := e.caretRect()
	e.moveCaret(1, false)
	pos1, _ := e.caretRect()
	assert.Greater(t, pos0.X, pos1.X, "caretRect follows the RTL visual order inside the word")
}

func TestRichEntry_BidiInlineImageSelection(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	e, img, w := bidiInlineEntry(t)
	defer w.Close()
	p := &e.provider

	th := e.Theme()
	innerPad := th.Size(theme.SizeNameInnerPadding)
	imgW := p.cachedSegmentVisual(img, 0).MinSize().Width
	imgLeft, ok := p.bidiInlineColumnX(0, 4, innerPad, true)
	require.True(t, ok)

	// Selecting only the image fills exactly the image.
	spans, ok := p.bidiInlineSelectionSpans(0, 4, 5, innerPad)
	require.True(t, ok)
	require.Len(t, spans, 1)
	assert.InDelta(t, imgLeft, spans[0][0], 0.6)
	assert.InDelta(t, imgW, spans[0][1]-spans[0][0], 0.6)

	// Selecting the first word only must not touch the image's interior.
	spans, ok = p.bidiInlineSelectionSpans(0, 0, 4, innerPad)
	require.True(t, ok)
	require.NotEmpty(t, spans)
	for _, sp := range spans {
		assert.LessOrEqual(t, sp[1], imgLeft+0.6, "text-only selection must stop at the image edge")
	}

	// Selecting everything covers the full visual line as one merged span.
	spans, ok = p.bidiInlineSelectionSpans(0, 0, 9, innerPad)
	require.True(t, ok)
	require.Len(t, spans, 1)
	assert.Greater(t, spans[0][1]-spans[0][0], imgW, "full selection spans text and image")

	// End-to-end: the selectable renderer builds a rectangle covering the image
	// when the selection is exactly the image's document position.
	e.moveCaret(4, false)
	e.moveCaret(5, true)
	e.Refresh()
	r, isSel := cache.Renderer(e.sel).(*selectableRenderer)
	require.True(t, isSel)
	require.NotEmpty(t, r.selections)
	found := false
	for _, rect := range r.selections {
		if rect.Size().Width >= imgW-0.6 && rect.Size().Width <= imgW+2.5 {
			found = true
		}
	}
	assert.True(t, found, "a selection rectangle of the image's width is drawn")
}

// TestRichEntry_InlineImageLTRFallback pins that pure-LTR rows keep the
// existing walk byte-identical: the bidi entry points must decline them.
func TestRichEntry_InlineImageLTRFallback(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	img := &ImageSegment{Image: image.NewNRGBA(image.Rect(0, 0, 24, 16)), InlineImage: true}
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: RichTextStyle{Inline: true}, Text: "ab"},
		img,
		&TextSegment{Style: RichTextStyle{Inline: true}, Text: "cd"},
	})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 80))

	innerPad := e.Theme().Size(theme.SizeNameInnerPadding)
	_, ok := e.provider.bidiInlineColumnX(0, 2, innerPad, true)
	assert.False(t, ok, "LTR rows stay on the inlineColumnX walk")
	_, ok = e.provider.bidiInlineSelectionSpans(0, 1, 4, innerPad)
	assert.False(t, ok, "LTR rows stay on the LTR selection path")
}
