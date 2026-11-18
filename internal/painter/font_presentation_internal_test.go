package painter

import (
	"image"
	"image/color"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/typesetting/shaping"
)

// The table must follow the Go standard library's Unicode version; when Go
// moves to a newer one, regenerate it with go generate (#1260).
func TestEmojiPresentationTableVersion(t *testing.T) {
	assert.Equal(t, unicode.Version, emojiPresentationVersion, "run go generate ./internal/painter/")

	for _, r := range "⭐⚡☕⚽❗⌚😀🦀" {
		assert.True(t, unicode.Is(emojiPresentation, r), "%q defaults to emoji", r)
	}
	for _, r := range "✔☑⚠♥❤↔▶◀✉ℹ☀©#1" {
		assert.False(t, unicode.Is(emojiPresentation, r), "%q defaults to text", r)
	}
}

func defaultFontMap(t *testing.T) *dynamicFontMap {
	t.Helper()
	d, ok := CachedFontFace(bellui.TextStyle{}, nil, nil).Fonts.(*dynamicFontMap)
	require.True(t, ok)
	require.NotNil(t, d.emoji, "the default chain carries the emoji face")
	return d
}

// A text-default symbol draws from a text face even though the emoji face,
// which comes before the symbol faces, covers it; an emoji-default one still
// draws as emoji (#1260).
func TestResolveFace_DefaultPresentation(t *testing.T) {
	d := defaultFontMap(t)

	// The symbols #1260 lists: each has a glyph in a bundled symbol face.
	for _, r := range "✔☑⚠♥❤↔↕↩↪▶◀▪▫⬆⬇➡⬅✖✉✂✏⌨ℹ☀" {
		_, inEmoji := d.emoji.NominalGlyph(r)
		require.True(t, inEmoji, "%q: the emoji face covers it, or the test proves nothing", r)
		assert.NotSame(t, d.emoji, d.ResolveFace(r), "%q defaults to text", r)
	}
	for _, r := range "⭐⚡☕⚽❗😀" {
		assert.Same(t, d.emoji, d.ResolveFace(r), "%q defaults to emoji", r)
	}
	// Plain text keeps the theme face.
	assert.Same(t, d.faces[0], d.ResolveFace('a'))
}

// A variation selector overrides the default, per occurrence: U+FE0F asks for
// emoji, U+FE0E for text (#1260).
func TestResolveFace_VariationSelectors(t *testing.T) {
	d := defaultFontMap(t)

	resolve := func(s string) []bool { // emoji face or not, per rune asked for
		text := []rune(s)
		var got []bool
		for _, run := range (&shaping.Segmenter{}).Split(shaping.Input{
			Text: text, RunEnd: len(text), Face: d.ResolveFace(' '), Size: float32ToFixed266(16),
		}, withPresentation(d, text)) {
			for _, r := range text[run.RunStart:run.RunEnd] {
				if r == ' ' || r == textPresentationSelector || r == emojiPresentationSelector {
					continue
				}
				got = append(got, run.Face == d.emoji)
			}
		}
		return got
	}

	assert.Equal(t, []bool{true}, resolve("❤️"), "FE0F makes a text-default rune emoji")
	assert.Equal(t, []bool{false}, resolve("⭐︎"), "FE0E makes an emoji-default rune text")
	assert.Equal(t, []bool{false, true, false}, resolve("❤ ❤️ ❤"), "each occurrence follows its own selector")
	assert.Equal(t, []bool{true, false, true}, resolve("⭐ ⭐︎ ⭐"))
	assert.Equal(t, []bool{false, false, true}, resolve("a✔ ✔️"))
}

// Rendered on the software painter in black, a text-presentation symbol has
// no colour, and an emoji-presentation one has (#1260).
func TestDrawString_EmojiPresentation(t *testing.T) {
	d := defaultFontMap(t)

	chroma := func(s string) int {
		img := image.NewNRGBA(image.Rect(0, 0, 120, 80))
		DrawString(img, s, color.Black, d, 40, 1, bellui.TextStyle{})
		most, inked := 0, false
		for i := 0; i < len(img.Pix); i += 4 {
			p := img.Pix[i : i+4]
			if p[3] == 0 {
				continue
			}
			inked = true
			hi := max(p[0], p[1], p[2])
			lo := min(p[0], p[1], p[2])
			most = max(most, int(hi)-int(lo))
		}
		require.True(t, inked, "%q drew nothing", s)
		return most
	}

	for _, s := range []string{"✔", "▶", "❤", "⚠", "⭐︎"} {
		assert.Zero(t, chroma(s), "%q draws as a plain glyph", s)
	}
	for _, s := range []string{"⭐", "❤️", "▶️"} {
		assert.Greater(t, chroma(s), 60, "%q draws as colour emoji", s)
	}
}
