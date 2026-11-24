package widget

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	paint "github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// A character no font draws is drawn inside the slot its carets give it, as
// its font's .notdef box (#1446). It was drawn as U+FFFD, about twice the
// slot's width, so it covered the spaces after it and the caret between two
// such characters sat in the middle of the first one's ink.
func TestRichText_MissingGlyphsDrawBetweenTheirCarets(t *testing.T) {
	test.NewTempApp(t)
	const m = "͸" // unassigned in Unicode, so no font has it
	f := paint.CachedFontFace(bellui.TextStyle{}, nil, nil).Fonts.ResolveFace([]rune(m)[0])
	_, has := f.NominalGlyph([]rune(m)[0])
	require.False(t, has, "a font on this build draws U+0378, so the test draws no missing glyph")

	text := "ab" + m + m + "  " + m + "  ab"
	st := RichTextStyleInline
	st.SizeName = theme.SizeNameHeadingText
	rt := NewRichText(&TextSegment{Style: st, Text: text})
	rt.Selectable = true
	w := test.NewWindow(rt)
	defer w.Close()
	w.Resize(geo.NewSize(400, 80))
	require.Len(t, rt.rowBounds, 1)

	pad := rt.Theme().Size(theme.SizeNameInnerPadding)
	size := rt.Theme().Size(theme.SizeNameText)
	origin := bellui.CurrentApp().Driver().AbsolutePositionForObject(rt)
	runes := []rune(text)
	carets := make([]float32, len(runes)+1)
	for col := range carets {
		carets[col] = origin.X + rt.lineSizeToColumn(col, 0, size, pad, true).Width + rt.selection.alignOffsetX(0)
	}
	_, rowH := rt.RowGeometry(0)
	cols := inkColumns(w.Canvas().Capture(), int(origin.Y), int(origin.Y+rowH))

	for col, r := range runes {
		if string(r) != m {
			continue
		}
		inked := false
		for x := int(carets[col]); x < int(carets[col+1]) && x < len(cols); x++ {
			inked = inked || cols[x]
		}
		assert.True(t, inked, "the missing character at column %d is drawn between its carets (%.1f, %.1f)", col, carets[col], carets[col+1])
	}
	// Ink lies inside the slot of a character that is not a space, give or
	// take a pixel for rounding.
	var stray []int
	for x, ink := range cols {
		if !ink {
			continue
		}
		inside := false
		for col, r := range runes {
			if r != ' ' && float32(x) >= carets[col]-1 && float32(x) < carets[col+1]+1 {
				inside = true
				break
			}
		}
		if !inside {
			stray = append(stray, x)
		}
	}
	assert.Empty(t, stray, "columns inked outside every character's caret slot (carets %v)", carets)
}
