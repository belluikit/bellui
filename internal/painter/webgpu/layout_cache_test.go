//go:build !gl && !wasm

package webgpu

import (
	"fmt"
	"image/color"
	"runtime"
	"testing"
	"time"

	"github.com/belluikit/typesetting/font"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/internal/cache"
	paint "github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// layoutPainter is the least of a painter layoutText reads: no GPU device.
func layoutPainter() *painter {
	return &painter{pixScale: 1, texScale: 1}
}

// BenchmarkLayoutTextHit is the per-frame cost of a text whose layout is
// already cached, which every batched text pays every frame (#483).
func BenchmarkLayoutTextHit(b *testing.B) {
	test.NewTempApp(b)
	p := layoutPainter()
	text := canvas.NewText("Batched 0123", color.White)
	text.TextSize = 16
	p.layoutText(text)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.layoutText(text)
	}
}

// monoTheme is the default theme with the monospace font as its text font.
type monoTheme struct{ bellui.Theme }

func (monoTheme) Font(bellui.TextStyle) bellui.Resource { return theme.DefaultTextMonospaceFont() }

// firstFace is the face a layout's first glyph was shaped with.
func firstFace(t *testing.T, l *textLayout) *font.Face {
	t.Helper()
	require.NotEmpty(t, l.glyphs)
	return l.glyphs[0].key.face
}

// A cached layout must not outlive the fonts it was shaped with: each way the
// fonts behind a text can change between frames shapes it again with the new
// face (#483).
func TestLayoutTextFollowsFontChanges(t *testing.T) {
	newText := func() *canvas.Text {
		text := canvas.NewText("Batched 0123", color.White)
		text.TextSize = 16
		return text
	}

	t.Run("theme font changed", func(t *testing.T) {
		a := test.NewTempApp(t)
		p, text := layoutPainter(), newText()
		before := p.layoutText(text)
		a.Settings().SetTheme(monoTheme{theme.DefaultTheme()})
		after := p.layoutText(text)
		assert.NotSame(t, before, after, "the text was not shaped again")
		assert.NotSame(t, firstFace(t, before), firstFace(t, after), "the new layout kept the old face")
	})

	t.Run("font cache cleared", func(t *testing.T) {
		test.NewTempApp(t)
		p, text := layoutPainter(), newText()
		before := p.layoutText(text)
		assert.Same(t, before, p.layoutText(text), "an unchanged text should hit the cache")
		paint.ClearFontCache()
		assert.NotSame(t, before, p.layoutText(text), "the text was not shaped again")
	})

	t.Run("theme overridden on the text", func(t *testing.T) {
		test.NewTempApp(t)
		p, text := layoutPainter(), newText()
		before := p.layoutText(text)
		gen := paint.FontCacheGeneration()
		cache.OverrideTheme(text, monoTheme{theme.DefaultTheme()})
		require.Equal(t, gen, paint.FontCacheGeneration(), "the override itself should not clear the font cache")
		after := p.layoutText(text)
		assert.NotSame(t, firstFace(t, before), firstFace(t, after), "the override's font was not used")
	})

	t.Run("font source set", func(t *testing.T) {
		test.NewTempApp(t)
		p, text := layoutPainter(), newText()
		before := p.layoutText(text)
		text.FontSource = theme.DefaultTextMonospaceFont()
		after := p.layoutText(text)
		assert.NotSame(t, firstFace(t, before), firstFace(t, after), "the font source was not used")
	})
}

// A text drawn from its cached layout does not look its fonts up, so after a
// minute the cache sweep forgets its font chain while the layout still holds
// the faces. The next text shaped then must get those same faces, not parse
// the font files again: a phone app playing a song grew from 1.6k to 43k live
// parsed-font objects in an hour, each play/pause icon change parsing a copy
// that the layouts and the glyph atlas kept (#1066). The static text carries
// an emoji so that its layout holds the emoji font too; a font nothing holds
// is released by the sweep and parsed again on its next use (#446).
func TestLayoutTextKeepsFacesAcrossSweeps(t *testing.T) {
	test.NewTempApp(t)
	p := layoutPainter()
	static := canvas.NewText("Now playing \U0001F3B5", color.White)
	static.TextSize = 16
	cache.OverrideTheme(static, theme.DefaultTheme())
	first := p.layoutText(static)
	held := map[*font.Face]bool{}
	for _, g := range first.glyphs {
		held[g.key.face] = true
	}
	require.Len(t, held, 2, "the static text should be shaped with the text and the emoji font")

	parses := paint.FontParses()
	for i := range 20 {
		paint.SweepFontCache(-time.Hour) // every chain is a minute old: only cache hits used them
		runtime.GC()
		require.True(t, first == p.layoutText(static), "the cached layout was dropped")

		changed := canvas.NewText(fmt.Sprintf("00:%02d \U0001F3B5", i), color.White)
		changed.TextSize = 16
		cache.OverrideTheme(changed, theme.DefaultTheme()) // a new theme scope, as each renderer under a ThemeOverride gets
		for _, g := range p.layoutText(changed).glyphs {
			assert.True(t, held[g.key.face], "round %d shaped with a second copy of a font still in use", i)
		}
	}
	assert.Equal(t, parses, paint.FontParses(), "fonts still in use were parsed again")
}
