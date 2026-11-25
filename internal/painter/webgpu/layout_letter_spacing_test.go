//go:build !gl && !wasm

package webgpu

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	paint "github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/test"
)

// The batched path places a letter-spaced text's glyphs where the caret
// layout puts them: its first glyph at the text's start and each one after it
// a gap further on (#1453). ShapeString handed it runs whose pen was already
// past every gap, while their glyphs advanced unspaced, so the text drew
// indented and unspaced here too.
func TestLayoutTextLetterSpacing(t *testing.T) {
	test.NewTempApp(t)
	const s = "Body Large - The quick brown fox jumps over the lazy dog"
	for _, scale := range []float32{1, 2} {
		for _, spacing := range []float32{0.5, 2} {
			p := layoutPainter()
			p.pixScale = scale
			text := canvas.NewText(s, color.White)
			text.TextSize = 16
			text.TextStyle.LetterSpacing = bellui.Ptr(spacing)
			l := p.layoutText(text)

			face := paint.CachedFontFace(text.TextStyle, nil, text)
			carets := paint.LineVisualLayout(face.Fonts, []rune(s), 16, text.TextStyle).Carets
			require.Len(t, l.glyphs, len([]rune(s)), "one glyph per rune")
			for i, g := range l.glyphs {
				// A run starts on a whole pixel, so a glyph sits up to one
				// pixel left of its exact caret.
				assert.InDelta(t, carets[i]*scale, g.x, 1, "scale %v, spacing %v: glyph %d is placed at its caret", scale, spacing, i)
			}
		}
	}
}
