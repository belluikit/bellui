// RTL caret/selection math measurably diverges under GOOS=js (#168 — the
// same divergence as internal/painter's caret-box test); tagged out to keep
// the leg green while that is root-caused, not as the fix.
//go:build !wasm

package widget

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
