package widget

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

func inlineImageSeg() *ImageSegment {
	return &ImageSegment{Image: image.NewNRGBA(image.Rect(0, 0, 24, 16)), InlineImage: true}
}

func caretXAt(e *RichEntry, offset int) float32 {
	e.moveCaret(offset, false)
	pos, _ := e.caretRect()
	return pos.X
}

// TestRichEntry_InlineImageStaysInRow verifies an inline image flows within the
// text row (text — image — text on one row) rather than forcing its own row, and
// that the document position space counts it as exactly one position (#665).
func TestRichEntry_InlineImageStaysInRow(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "ab"},
		inlineImageSeg(),
		&TextSegment{Style: normalStyle(), Text: "cd"},
	})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 80))

	assert.Equal(t, 1, e.provider.rows(), "text + inline image + text stay on one row")
	assert.Equal(t, 1, e.doc.rowCount(), "the row model agrees")
	assert.Equal(t, 5, e.doc.len(), "ab(2) + image(1) + cd(2) = 5 positions")
	assert.False(t, e.doc.rowIsImage(0), "a mixed row is not a block-image row")
}

// TestRichEntry_InlineImageCaretWalksOverImage is the core geometry check: the
// caret x advances monotonically across the row and jumps by the image's width
// as it steps over the inline image — lineSizeToColumn would measure through it.
func TestRichEntry_InlineImageCaretWalksOverImage(t *testing.T) {
	test.NewTempApp(t)
	img := inlineImageSeg()
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "ab"},
		img,
		&TextSegment{Style: normalStyle(), Text: "cd"},
	})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 80))

	var xs []float32
	for off := 0; off <= e.doc.len(); off++ {
		xs = append(xs, caretXAt(e, off))
	}
	for i := 1; i < len(xs); i++ {
		assert.Greater(t, xs[i], xs[i-1], "caret x is monotonic across offset %d", i)
	}
	// The step from before the image (offset 2) to after it (offset 3) must be at
	// least the image's rendered width — proof the walk accounts for the atom.
	imgW := e.provider.cachedSegmentVisual(img, 0).MinSize().Width
	require.Positive(t, imgW)
	require.Greater(t, imgW, float32(0))
	assert.GreaterOrEqual(t, xs[3]-xs[2], imgW*0.9, "stepping over the image advances by ~its width")
}

// TestRichEntry_InlineImageRoundTrip walks every offset and asserts the
// offset<->(row,col) mapping is self-consistent with inline images present, both
// wrap-off and wrap-on.
func TestRichEntry_InlineImageRoundTrip(t *testing.T) {
	test.NewTempApp(t)
	for _, wrap := range []bellui.TextWrap{bellui.TextWrapOff, bellui.TextWrapWord} {
		e := NewRichEntry()
		e.Wrapping = wrap
		e.SetSegments([]RichTextSegment{
			&TextSegment{Style: normalStyle(), Text: "hello "},
			inlineImageSeg(),
			&TextSegment{Style: normalStyle(), Text: " world foo bar baz"},
		})
		w := test.NewWindow(e)
		w.Resize(geo.NewSize(120, 200))

		rows := e.doc.activeRowModel()
		require.Equal(t, e.provider.rows(), len(rows), "wrap=%v: model matches provider rows", wrap)
		assert.Equal(t, e.doc.len(), rows[len(rows)-1].end, "wrap=%v: covers all positions", wrap)
		for off := 0; off <= e.doc.len(); off++ {
			row, col := e.doc.offsetToRowCol(off)
			require.Less(t, row, len(rows))
			back := e.doc.rowColToOffset(row, col)
			r2, c2 := e.doc.offsetToRowCol(back)
			assert.Equal(t, row, r2, "wrap=%v idempotent row at offset %d", wrap, off)
			assert.Equal(t, col, c2, "wrap=%v idempotent col at offset %d", wrap, off)
		}
		w.Close()
	}
}

// TestRichEntry_InsertInlineImage inserts an inline image at the caret and
// confirms it lands in the row, one position wide, caret after it.
func TestRichEntry_InsertInlineImage(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{&TextSegment{Style: normalStyle(), Text: "abcd"}})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 80))

	e.moveCaret(2, false)
	e.InsertInlineImage(image.NewNRGBA(image.Rect(0, 0, 16, 16)))

	assert.Equal(t, 5, e.doc.len(), "one position added")
	assert.Equal(t, 3, e.caret, "caret lands after the inserted image")
	assert.Equal(t, 1, e.provider.rows(), "image flows inline, no new row")
	require.Len(t, e.Segments(), 3)
	img, ok := e.Segments()[1].(*ImageSegment)
	require.True(t, ok)
	assert.True(t, img.InlineImage, "inserted as an inline image")

	e.Undo()
	assert.Equal(t, "abcd", e.String(), "undo removes the inline image")
	assert.Len(t, e.Segments(), 1)
}

// TestRichEntry_BlockImageStillOwnsRow guards that the default block image path
// is unchanged: it takes its own row and reports as an image row.
func TestRichEntry_BlockImageStillOwnsRow(t *testing.T) {
	test.NewTempApp(t)
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "ab"},
		&ImageSegment{Image: image.NewNRGBA(image.Rect(0, 0, 16, 16))}, // block (default)
		&TextSegment{Style: normalStyle(), Text: "cd"},
	})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 120))

	require.Equal(t, 3, e.provider.rows(), "block image is on its own row")
	assert.Equal(t, 3, e.doc.rowCount())
	assert.True(t, e.doc.rowIsImage(1), "the middle row is a block image row")
}

// TestRichEntry_SelectionHighlightSpansInlineImage is the #893 fix: a selection
// that crosses an inline image highlights the image's full width. Before, the
// highlight went through lineSizeToColumn (text-rune columns) and mis-measured a
// document-space selection, ending short by the image's width.
func TestRichEntry_SelectionHighlightSpansInlineImage(t *testing.T) {
	test.NewTempApp(t)
	img := &ImageSegment{Image: image.NewNRGBA(image.Rect(0, 0, 40, 16)), InlineImage: true}
	e := NewRichEntry()
	e.SetSegments([]RichTextSegment{
		&TextSegment{Style: normalStyle(), Text: "ab"},
		img,
		&TextSegment{Style: normalStyle(), Text: "cd"},
	})
	w := test.NewWindow(e)
	defer w.Close()
	w.Resize(geo.NewSize(400, 80))

	// Select positions 0..3 ("ab" + the image) — a partial cross of the image.
	e.anchor, e.caret, e.selecting = 0, 3, true
	e.syncSel()
	r := e.sel.CreateRenderer().(*selectableRenderer)
	r.Refresh()
	require.Len(t, r.selections, 1, "one selection rectangle on the single row")
	selW := r.selections[0].Size().Width

	// The caret x span for the same range uses the atom-aware walk (already
	// correct); the highlight must match it.
	x0 := caretXAt(e, 0)
	x3 := caretXAt(e, 3)
	caretSpan := x3 - x0

	imgW := e.provider.cachedSegmentVisual(img, 0).MinSize().Width
	assert.InDelta(t, caretSpan, selW, 2.0, "highlight width matches the caret span across the image")
	assert.GreaterOrEqual(t, selW, imgW, "highlight covers at least the image width")
}
