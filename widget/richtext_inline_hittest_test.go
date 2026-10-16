package widget

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// Click hit-testing on rows that mix text with inline image atoms must be
// atom-aware in both directions (pre-rebrand tracker #983): the LTR prefix scan measured *through*
// the image (every hit after it one image-width off) and the bidi hit-test
// returned box-blind text-only columns (consumed as document columns by
// RichEntry, off by one per preceding atom). These tests click at known x on
// such rows and assert the resolved column.

// ltrInlineEntry builds a RichEntry with one LTR row: "ab", inline image, "cd"
// (document positions: 0=a, 1=b, 2=before image, 3=after image/"c", 4="d",
// 5=end; text-only columns: a=0,b=1,c=2,d=3,end=4).
func ltrInlineEntry(t *testing.T) (*RichEntry, *ImageSegment, bellui.Window) {
	t.Helper()
	img := &ImageSegment{Image: image.NewNRGBA(image.Rect(0, 0, 24, 16)), InlineImage: true}
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: RichTextStyle{Inline: true}, Text: "ab"},
		img,
		&TextSegment{Style: RichTextStyle{Inline: true}, Text: "cd"},
	})
	w := test.NewWindow(e)
	w.Resize(geo.NewSize(400, 80))
	return e, img, w
}

func TestRichEntry_InlineImageHitTestLTR(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	e, img, w := ltrInlineEntry(t)
	defer w.Close()
	p := &e.provider

	require.Equal(t, 1, p.rows())
	require.Equal(t, 5, e.doc.len())

	innerPad := e.Theme().Size(theme.SizeNameInnerPadding)
	imgW := p.cachedSegmentVisual(img, 0).MinSize().Width
	require.Positive(t, imgW)
	off := innerPad - p.inset.Width

	// The bidi path declines a pure-LTR row (no RTL content).
	_, _, _, ok := p.bidiInlineHitTest(0, off, innerPad)
	assert.False(t, ok, "bidi hit-test declines a pure-LTR row")

	// inlineColumnX gives the forward caret x for each document column; clicking
	// at that x must resolve back to the same column (the inverse round-trips).
	x := func(docCol int) float32 {
		v, xok := p.inlineColumnX(0, docCol, 0, innerPad)
		require.True(t, xok)
		return v
	}
	beforeImg := x(2) // caret x just before the image
	afterImg := x(3)  // caret x just after the image ( = start of "cd")
	assert.InDelta(t, imgW, afterImg-beforeImg, 0.6, "the image occupies its width between docs 2 and 3")

	hit := func(clickX float32) (textCol, docCol int) {
		tc, dc, _, hok := p.inlineColumnAt(0, clickX, innerPad)
		require.True(t, hok, "LTR atom-aware hit-test must engage")
		return tc, dc
	}

	// Click before the image (left quarter of the image box): document col 2,
	// text-only col 2.
	tc, dc := hit(beforeImg + imgW*0.25)
	assert.Equal(t, 2, dc, "click on the image's leading half resolves before it")
	assert.Equal(t, 2, tc)

	// Click on the image's trailing half: document col 3 (after the image),
	// text-only col still 2 (the atom has no text column).
	_, dc = hit(beforeImg + imgW*0.75)
	assert.Equal(t, 3, dc, "click on the image's trailing half resolves after it")

	// Click on the text following the image ("c") must land on doc col 3/4, NOT
	// past the end — the before pre-rebrand tracker #983 scan measured through the image and overshot.
	cx := x(3) // start of "c"
	_, dc = hit(cx + 0.5)
	assert.GreaterOrEqual(t, dc, 3)
	assert.LessOrEqual(t, dc, 4, "text after the image resolves to its own columns, not past the end")

	// End-to-end through getRowCol (RichEntry stores document columns): clicking
	// on the image's trailing half places the caret after the image.
	e.moveCaret(0, false)
	y, _ := e.caretRect()
	row, col, _ := e.sel.getRowCol(geo.NewPos(beforeImg+imgW*0.75, y.Y+2))
	assert.Equal(t, 0, row)
	assert.Equal(t, 3, col, "getRowCol returns the document column after the image")
}

func TestRichEntry_BidiInlineImageHitTest(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	e, img, w := bidiInlineEntry(t)
	defer w.Close()
	p := &e.provider

	require.Equal(t, 1, p.rows())
	require.Equal(t, 9, e.doc.len())

	innerPad := e.Theme().Size(theme.SizeNameInnerPadding)
	imgW := p.cachedSegmentVisual(img, 0).MinSize().Width
	require.Positive(t, imgW)

	// The bidi path engages for the RTL+image row.
	imgBefore, ok := p.bidiInlineColumnX(0, 4, innerPad, true) // caret before the image
	require.True(t, ok)
	imgAfter, ok := p.bidiInlineColumnX(0, 5, innerPad, true) // caret after the image
	require.True(t, ok)
	// The image occupies its width between docs 4 and 5, regardless of RTL flow.
	imgLeft := imgBefore
	if imgAfter < imgLeft {
		imgLeft = imgAfter
	}
	assert.InDelta(t, imgW, absf32(imgAfter-imgBefore), 0.6)

	hit := func(clickX float32) (textCol, docCol int) {
		tc, dc, _, hok := p.bidiInlineHitTest(0, clickX, innerPad)
		require.True(t, hok, "bidi atom-aware hit-test must engage")
		return tc, dc
	}

	// A click inside the image resolves to the atom's own document position
	// (4 or 5), never a neighbouring text column — HitTestAffinity would snap to
	// the nearest text caret and miss the atom entirely.
	_, dc := hit(imgLeft + imgW*0.5)
	assert.Contains(t, []int{4, 5}, dc, "a click on the image resolves to the atom's document position")

	// A click well inside the leading RTL word (visually to the right of the
	// image, since RTL flows right-to-left) resolves into that word's columns
	// (document 0..4), not into the trailing word.
	wordRightX, ok := p.bidiInlineColumnX(0, 2, innerPad, true)
	require.True(t, ok)
	_, dc = hit(wordRightX)
	assert.LessOrEqual(t, dc, 4, "a click in the leading RTL word stays in its columns")

	// A click well inside the trailing RTL word (visually to the left) resolves
	// into the trailing word's columns (document 5..9).
	wordLeftX, ok := p.bidiInlineColumnX(0, 7, innerPad, true)
	require.True(t, ok)
	_, dc = hit(wordLeftX)
	assert.GreaterOrEqual(t, dc, 5, "a click in the trailing RTL word stays in its columns")

	// End-to-end: getRowCol returns document columns for the editor host.
	e.moveCaret(4, false)
	y, _ := e.caretRect()
	row, col, _ := e.sel.getRowCol(geo.NewPos(imgLeft+imgW*0.5, y.Y+2))
	assert.Equal(t, 0, row)
	assert.Contains(t, []int{4, 5}, col, "getRowCol on the image returns the atom's document column")
}

// TestSelectableRichText_InlineImageHitTest verifies that a *plain* selectable
// RichText (host == nil, text-only column model) keeps its text-only columns but
// hit-tests with atom-aware x: clicking on the text after an inline image lands
// on that text's column, not one image-width past it (the before pre-rebrand tracker #983 bug).
func TestSelectableRichText_InlineImageHitTest(t *testing.T) {
	test.NewTempApp(t)
	lang.SetLocaleForTests(t, "en-US")
	img := &ImageSegment{Image: image.NewNRGBA(image.Rect(0, 0, 24, 16)), InlineImage: true}
	rt := NewRichText(
		&TextSegment{Style: RichTextStyle{Inline: true}, Text: "ab"},
		img,
		&TextSegment{Style: RichTextStyle{Inline: true}, Text: "cd"},
	)
	rt.Selectable = true
	w := test.NewWindow(rt)
	defer w.Close()
	w.Resize(geo.NewSize(400, 80))

	require.NotNil(t, rt.selection)
	sel := &rt.selection.selectable
	require.Nil(t, sel.host, "plain selectable has no editor host")

	innerPad := rt.Theme().Size(theme.SizeNameInnerPadding)
	imgW := rt.cachedSegmentVisual(img, 0).MinSize().Width
	require.Positive(t, imgW)

	// Text-only columns: a=0,b=1,c=2,d=3,end=4; the image has no text column.
	// Forward x at text col via inlineColumnX (document col 3 == start of "c").
	cx, ok := rt.inlineColumnX(0, 3, 0, innerPad)
	require.True(t, ok)

	// Direct: inlineColumnAt returns the text-only column too.
	tc, _, _, ok := rt.inlineColumnAt(0, cx+0.5, innerPad)
	require.True(t, ok)
	assert.GreaterOrEqual(t, tc, 2)
	assert.LessOrEqual(t, tc, 3, "text after the image maps to its own text column, not past the end")

	// End-to-end through the host-nil path: clicking on "c" resolves to text col
	// 2 or 3, never 4 (the pre-fix through-image scan overshot to the row end).
	e2eY := innerPad + 2
	_, col, _ := sel.getRowCol(geo.NewPos(cx+0.5, e2eY))
	assert.GreaterOrEqual(t, col, 2)
	assert.LessOrEqual(t, col, 3, "selectable text-only column after the image is not off by the image width")
}

func absf32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
