package painter

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"
	"unicode/utf8"

	"github.com/belluikit/typesetting/shaping"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	intTest "github.com/belluikit/bellui/internal/test"
)

// noFontRune is unassigned in Unicode, so no font on any build draws it: the
// shaper gives it glyph 0 in whatever face the font map falls back to.
const noFontRune = '͸'

// requireMissing fails unless every rune of s that is not a space is shaped
// as a missing glyph (GlyphID 0) with f, so a test meant for missing glyphs
// cannot pass by drawing real ones.
func requireMissing(t *testing.T, f shaping.Fontmap, s string) {
	t.Helper()
	n := 0
	walkString(f, s, float32ToFixed266(40), bellui.TextStyle{}, new(float32), 1, func(out shaping.Output, _ float32) {
		for _, g := range out.Glyphs {
			if g.GlyphID == 0 {
				n++
			}
		}
	})
	want := 0
	for _, r := range s {
		if r != ' ' {
			want++
		}
	}
	require.Equal(t, want, n, "every letter of %q is a missing glyph on this build", s)
}

// slot is the span [x0, x1) of one rune's advance, in image columns.
type slot struct {
	x0, x1  float32
	missing bool
	r       rune
}

// prefixSlots returns each rune's slot in s drawn from x = at, from the
// measured widths of the prefixes of s.
func prefixSlots(f shaping.Fontmap, s string, size, at float32) []slot {
	var slots []slot
	prev := at
	for i, r := range s {
		end, _ := MeasureString(f, s[:i+utf8.RuneLen(r)], size, bellui.TextStyle{})
		slots = append(slots, slot{x0: prev, x1: at + end.Width, missing: r != ' ' && r != 'a' && r != 'b', r: r})
		prev = at + end.Width
	}
	return slots
}

// inkCols reports which columns of img hold clear ink: alpha at least a sixth
// of full, so a glyph's faint antialiased edge is left out.
func inkCols(img *image.NRGBA) []bool {
	b := img.Bounds()
	cols := make([]bool, b.Dx())
	for x := 0; x < b.Dx(); x++ {
		for y := 0; y < b.Dy(); y++ {
			if img.NRGBAAt(b.Min.X+x, b.Min.Y+y).A > 42 {
				cols[x] = true
				break
			}
		}
	}
	return cols
}

// assertInkInSlots checks that each missing glyph's slot holds ink, so it is
// drawn, and that no ink lies outside the slots of the drawn runes, give or
// take a pixel for rounding: a glyph wider than its advance spills into the
// space after it.
func assertInkInSlots(t *testing.T, cols []bool, slots []slot, msg string) {
	t.Helper()
	for _, s := range slots {
		if !s.missing {
			continue
		}
		inked := false
		for x := int(math.Floor(float64(s.x0))); x < int(math.Ceil(float64(s.x1))) && x < len(cols); x++ {
			inked = inked || x >= 0 && cols[x]
		}
		assert.True(t, inked, "%s: the missing glyph in [%.1f, %.1f) is drawn", msg, s.x0, s.x1)
	}
	var stray []int
	for x, ink := range cols {
		if !ink {
			continue
		}
		inside := false
		for _, s := range slots {
			if s.r != ' ' && float32(x) >= s.x0-1 && float32(x) < s.x1+1 {
				inside = true
				break
			}
		}
		if !inside {
			stray = append(stray, x)
		}
	}
	assert.Empty(t, stray, "%s: columns inked outside every glyph's advance (slots %v)", msg, slots)
}

// A character no font draws is drawn as its face's .notdef glyph, which fits
// the advance the line is measured and its carets placed with (#1446). It was
// drawn as U+FFFD: 41 px wide in a 17.7 px slot at size 40, so it covered the
// space after it, and a run of them drew as one solid block.
func TestDrawString_MissingGlyphsDrawInsideTheirAdvance(t *testing.T) {
	ascii := &intTest.FontMap{CachedFontFace(bellui.TextStyle{}, nil, nil).Fonts.ResolveFace(' ')}
	for _, s := range []string{"aसस  स  a", "सससस", "a  स"} {
		requireMissing(t, ascii, stripLatin(s))
		img := image.NewNRGBA(image.Rect(0, 0, 400, 80))
		DrawString(img, s, color.Black, ascii, 40, 1, bellui.TextStyle{})
		assertInkInSlots(t, inkCols(img), prefixSlots(ascii, s, 40, 0), fmt.Sprintf("%q", s))
	}
}

// The same holds for styled runs (canvas.StyledText), which draw a missing
// glyph through drawColoredOutput, in its run's colour.
func TestDrawStyledRuns_MissingGlyphsDrawInsideTheirAdvance(t *testing.T) {
	fonts := CachedFontFace(bellui.TextStyle{}, nil, nil).Fonts
	m := string(noFontRune)
	requireMissing(t, fonts, m+m)
	runs := []canvas.TextRun{
		{Text: "ab  ", TextSize: 40},
		{Text: m + m, TextSize: 40, Color: color.NRGBA{R: 255, A: 255}},
		{Text: "  " + m + "  ", TextSize: 40},
		{Text: "ab", TextSize: 40},
	}
	_, advances := MeasureStyledRuns(runs, 40)
	var slots []slot
	at := float32(0)
	for i, run := range runs {
		slots = append(slots, prefixSlots(fonts, run.Text, 40, at)...)
		at += advances[i]
	}
	img := image.NewNRGBA(image.Rect(0, 0, 500, 80))
	DrawStyledRuns(img, runs, color.Black, 40, 1)
	assertInkInSlots(t, inkCols(img), slots, "styled runs")
}

// And for TextOnPath, which drew U+FFFD into an image sized from the missing
// glyph's advance, so it was cropped as well as spilling a few pixels over.
// On a straight path each glyph sits in its advance along the line.
func TestDrawTextOnPath_MissingGlyphsDrawInsideTheirAdvance(t *testing.T) {
	fonts := CachedFontFace(bellui.TextStyle{}, nil, nil).Fonts
	m := string(noFontRune)
	s := m + m + "  " + m
	requireMissing(t, fonts, s)
	top := canvas.NewTextOnPath(s, color.Black, func(p *canvas.TextOnPath) {
		p.MoveTo(0, 0)
		p.LineTo(400, 0)
	})
	top.TextSize = 40
	img := DrawTextOnPath(top, 1)
	require.NotNil(t, img)
	total, _ := MeasureString(fonts, s, 40, bellui.TextStyle{})
	// The image pads the line by the same amount on each side.
	pad := (float32(img.Bounds().Dx()) - total.Width) / 2
	assertInkInSlots(t, inkCols(img), prefixSlots(fonts, s, 40, pad), "text on path")
}

// stripLatin drops the ASCII letters and spaces of s, leaving the runes the
// ASCII-only font map lacks.
func stripLatin(s string) string {
	out := []rune{}
	for _, r := range s {
		if r > 0x7f {
			out = append(out, r)
		}
	}
	return string(out)
}
