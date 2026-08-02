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
