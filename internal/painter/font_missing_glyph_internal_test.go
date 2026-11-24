package painter

import (
	"testing"

	"github.com/belluikit/typesetting/shaping"
	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	intTest "github.com/belluikit/bellui/internal/test"
)

// A missing glyph advances the line once. shapeCallback moved x past it and
// also added its advance to the run that followed, so each run of missing
// glyphs was measured one glyph wider than the shaper laid it out, and the
// text after it was drawn that much further right. The caret layout
// (LineVisualLayout) counts each advance once, so on a build with no font for
// a script (wasm has no system fonts) RichText's carets and its bidi placement
// (#1441) disagreed with the drawn text.
func TestWalkString_MissingGlyphAdvancesOnce(t *testing.T) {
	f := CachedFontFace(bellui.TextStyle{}, nil, nil)
	ascii := &intTest.FontMap{f.Fonts.ResolveFace(' ')} // no Devanagari: स is missing
	width := func(s string) float32 {
		sz, _ := MeasureString(ascii, s, 40, bellui.TextStyle{})
		return sz.Width
	}
	a, missing := width("a"), width("स")
	if !assert.Greater(t, missing, float32(0), "the missing glyph has an advance") {
		return
	}

	for _, s := range []string{"aसa", "aससa", "सa", "aस"} {
		var xs []float32
		var adv float32
		walkString(ascii, s, float32ToFixed266(40), bellui.TextStyle{}, &adv, 1, func(_ shaping.Output, x float32) {
			xs = append(xs, x)
		})
		var want float32
		for _, r := range s {
			if r == 'a' {
				want += a
			} else {
				want += missing
			}
		}
		assert.InDelta(t, want, width(s), 0.01, "%q is measured as its glyphs' advances", s)
		assert.InDelta(t, LineVisualLayout(ascii, []rune(s), 40, bellui.TextStyle{}).Width, width(s), 0.01,
			"%q is measured as the caret layout lays it out", s)
		if s[len(s)-1] == 'a' {
			assert.InDelta(t, want-a, xs[len(xs)-1], 0.01, "%q draws its last letter after the missing glyphs, once each", s)
		}
	}
}
