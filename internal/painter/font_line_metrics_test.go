package painter_test

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/belluikit/typesetting/shaping"
	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/test"
)

// A symbol drawn from a bundled symbol font leaves a line as tall as the text
// font's, with the same baseline: those fonts keep Noto's tall line metrics
// (up to 2.05 em against the text font's 1.17), which their glyphs do not
// need, and they made a "✓" or "→" line 45-75% taller, its text lower, and a
// one-line entry overflow (#1412).
func TestBundledSymbolsKeepTheLineHeight(t *testing.T) {
	test.NewTempApp(t)
	plain, plainBase := painter.RenderedTextSize("clear x", 14, bellui.TextStyle{}, nil)
	for _, s := range []string{"clear ✓", "clear ✕", "clear ★", "clear →", "clear ⌘", "clear ☐", "✓", "→ next"} {
		size, base := painter.RenderedTextSize(s, 14, bellui.TextStyle{}, nil)
		assert.Equal(t, plain.Height, size.Height, "%q: height", s)
		assert.Equal(t, plainBase, base, "%q: baseline", s)
	}
}

// Other fallback faces still grow the line, as they did (#314): the bundled
// emoji face is taller than the text font.
func TestEmojiStillGrowsTheLine(t *testing.T) {
	test.NewTempApp(t)
	plain, _ := painter.RenderedTextSize("clear x", 14, bellui.TextStyle{}, nil)
	emoji, _ := painter.RenderedTextSize("clear 😀", 14, bellui.TextStyle{}, nil)
	assert.Greater(t, emoji.Height, plain.Height)
}

// Text is drawn on the line's baseline, the one it is measured with, not on
// its first run's: a line starting with a symbol or an emoji was drawn on that
// font's ascent, and one ending with an emoji on the text font's (#1412).
func TestDrawnBaselineIsTheLineBaseline(t *testing.T) {
	test.NewTempApp(t)
	fonts := painter.CachedFontFace(bellui.TextStyle{}, nil, nil).Fonts
	for _, s := range []string{"x ab", "✓ ab", "→ ab", "😀 ab", "ab 😀"} {
		_, measured := painter.RenderedTextSize(s, 14, bellui.TextStyle{}, nil)
		drawn := painter.ShapeString(fonts, s, 14, 1, bellui.TextStyle{}, func(shaping.Output, float32) {})
		assert.Equal(t, int(math.Ceil(float64(measured))), drawn, "%q", s)
	}

	// DrawString places "ab" on the same row whatever comes before it.
	bottom := func(s string) int {
		size, _ := painter.RenderedTextSize(s, 28, bellui.TextStyle{}, nil)
		img := image.NewNRGBA(image.Rect(0, 0, int(size.Width)+4, int(size.Height)+4))
		painter.DrawString(img, s, color.Black, fonts, 28, 1, bellui.TextStyle{})
		x0 := img.Bounds().Dx() * 3 / 5 // right of the first glyph: "ab"
		for y := img.Bounds().Dy() - 1; y >= 0; y-- {
			for x := x0; x < img.Bounds().Dx(); x++ {
				if img.NRGBAAt(x, y).A > 128 {
					return y
				}
			}
		}
		return -1
	}
	assert.Equal(t, bottom("x ab"), bottom("✓ ab"), "a leading ✓ does not move the text down")
}
