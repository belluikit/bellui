package painter

import (
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/belluikit/typesetting/shaping"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
)

// letterSpacingLine is the MD3 Body Large sample the gallery's Typography page
// showed indented (#1453): 56 runes, one glyph each in the default font.
const letterSpacingLine = "Body Large - The quick brown fox jumps over the lazy dog"

func spaced(s float32) bellui.TextStyle {
	return bellui.TextStyle{LetterSpacing: bellui.Ptr(s)}
}

// inkSpan returns the first and last column of img holding clear ink.
func inkSpan(t *testing.T, img *image.NRGBA) (from, to int) {
	t.Helper()
	from, to = -1, -1
	for x, ink := range inkCols(img) {
		if ink {
			if from < 0 {
				from = x
			}
			to = x
		}
	}
	require.GreaterOrEqual(t, from, 0, "the text drew no ink")
	return from, to
}

// glyphXs returns the pen x of each glyph walkString lays s out with, at
// scale 1, keyed by the rune it starts.
func glyphXs(f shaping.Fontmap, s string, size float32, style bellui.TextStyle) map[int]float32 {
	xs := map[int]float32{}
	walkString(f, s, float32ToFixed266(size), style, new(float32), 1, func(out shaping.Output, x float32) {
		for _, g := range out.Glyphs {
			xs[g.TextIndex()] = x
			x += fixed266ToFloat32(g.Advance)
		}
	})
	return xs
}

// Letter spacing goes between the glyphs: the text still starts where it
// starts, and its ink grows by the spacing once per gap (#1453). It was added
// to the pen once per glyph before a run was drawn, so a run started late by
// all of its gaps and its glyphs were drawn at their plain advances.
func TestDrawString_LetterSpacingSpacesGlyphs(t *testing.T) {
	f := CachedFontFace(bellui.TextStyle{}, nil, nil).Fonts
	gaps := float32(len([]rune(letterSpacingLine)) - 1)
	draw := func(spacing float32) (from, to int, width float32) {
		style := spaced(spacing)
		size, _ := MeasureString(f, letterSpacingLine, 16, style)
		img := image.NewNRGBA(image.Rect(0, 0, int(size.Width)+8, 40))
		DrawString(img, letterSpacingLine, color.Black, f, 16, 1, style)
		from, to = inkSpan(t, img)
		return from, to, size.Width
	}
	plainFrom, plainTo, plainWidth := draw(0)
	for _, spacing := range []float32{0.5, 2, -0.25} {
		from, to, width := draw(spacing)
		msg := fmt.Sprintf("spacing %v", spacing)
		assert.InDelta(t, plainWidth+spacing*gaps, width, 0.5, "%s: measured once per gap", msg)
		assert.InDelta(t, plainFrom, from, 1, "%s: the first glyph is drawn at the text's start", msg)
		assert.InDelta(t, float32(plainTo-plainFrom)+spacing*gaps, float32(to-from), 2, "%s: the ink grows once per gap", msg)
	}
}

// The caret layouts, canvas.Text's and RichText's, put each rune where its
// glyph is drawn, letter-spaced.
func TestLineVisualLayout_LetterSpacingMatchesDraw(t *testing.T) {
	f := CachedFontFace(bellui.TextStyle{}, nil, nil).Fonts
	for _, spacing := range []float32{0.5, 2} {
		style := spaced(spacing)
		xs := glyphXs(f, letterSpacingLine, 16, style)
		layout := LineVisualLayout(f, []rune(letterSpacingLine), 16, style)
		require.Len(t, xs, len([]rune(letterSpacingLine)), "one glyph per rune")
		for i, x := range xs {
			assert.InDelta(t, x, layout.Carets[i], 0.01, "spacing %v: rune %d's caret is where its glyph is drawn", spacing, i)
		}
		styled := StyledLineVisualLayout([]StyledRun{{Runes: []rune(letterSpacingLine), TextSize: 16, Style: style}})
		require.Len(t, styled.Carets, len(layout.Carets))
		for i, x := range xs {
			assert.InDelta(t, x, styled.Carets[i], 0.01, "spacing %v: rune %d's RichText caret is where its glyph is drawn", spacing, i)
		}
		size, _ := MeasureString(f, letterSpacingLine, 16, style)
		assert.InDelta(t, size.Width, layout.Width, 0.01, "spacing %v: the caret layout is as wide as the measure", spacing)
		assert.InDelta(t, size.Width, styled.Width, 0.01, "spacing %v: the RichText caret layout is as wide as the measure", spacing)
	}
}

// A combining mark stays on its letter: the spacing goes after the whole
// cluster, not between the letter and its mark. x has no precomposed form with
// an acute, so the mark is a glyph of its own in x's cluster.
func TestWalkString_LetterSpacingKeepsMarksOnTheirLetter(t *testing.T) {
	f := CachedFontFace(bellui.TextStyle{}, nil, nil).Fonts
	const s = "ax\u0301a"
	glyphs := func(style bellui.TextStyle) (clusters []int, xs []float32) {
		walkString(f, s, float32ToFixed266(40), style, new(float32), 1, func(out shaping.Output, x float32) {
			for _, g := range out.Glyphs {
				clusters = append(clusters, g.TextIndex())
				xs = append(xs, x)
				x += fixed266ToFloat32(g.Advance)
			}
		})
		return clusters, xs
	}
	clusters, plain := glyphs(bellui.TextStyle{})
	require.Equal(t, []int{0, 1, 1, 3}, clusters, "the acute is a glyph of its own in x's cluster")
	_, gap := glyphs(spaced(4))
	assert.InDelta(t, plain[2]-plain[1], gap[2]-gap[1], 0.01, "the mark keeps its place on the x")
	assert.InDelta(t, plain[3]+8, gap[3], 0.01, "the two clusters before the last a are each followed by one gap")

	layout := LineVisualLayout(f, []rune(s), 40, spaced(4))
	assert.InDelta(t, gap[3], layout.Carets[3], 0.01, "the caret after the accented x is where the next glyph is drawn")
}

// Text on a path is letter-spaced as well: it places each glyph by its
// advance, which now carries the gap. It drew unspaced before #1453, since it
// read the advances and not the pen walkString moved on.
func TestDrawTextOnPath_LetterSpacing(t *testing.T) {
	const s = "spaced out"
	draw := func(spacing float32) (from, to int) {
		top := canvas.NewTextOnPath(s, color.Black, func(p *canvas.TextOnPath) {
			p.MoveTo(0, 0)
			p.LineTo(600, 0)
		})
		top.TextSize = 40
		top.TextStyle = spaced(spacing)
		img := DrawTextOnPath(top, 1)
		require.NotNil(t, img)
		return inkSpan(t, img)
	}
	plainFrom, plainTo := draw(0)
	from, to := draw(6)
	gaps := float32(len(s) - 1)
	assert.InDelta(t, float32(plainTo-plainFrom)+6*gaps, float32(to-from), 2, "the ink grows once per gap")
}
