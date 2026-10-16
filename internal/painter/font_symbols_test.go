package painter

import (
	"testing"

	"github.com/belluikit/typesetting/font"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

// TestBundledFacesCoverCommonSymbols locks in host-independent text metrics
// for common UI symbols (pre-rebrand tracker #686): every rune here must be covered by a
// compiled-in face — the theme fonts, the emoji font or the bundled symbol
// fallbacks — so it can never fall through to the host system-font scan,
// where per-OS fonts would give per-OS metrics (and tofu on wasm).
func TestBundledFacesCoverCommonSymbols(t *testing.T) {
	resources := []bellui.Resource{
		theme.DefaultTextFont(),
		theme.DefaultTextBoldFont(),
		theme.DefaultTextMonospaceFont(),
	}
	if emoji := theme.DefaultEmojiFont(); emoji != nil {
		resources = append(resources, emoji)
	}
	resources = append(resources, theme.DefaultSymbolsFallbackFonts()...)

	var faces []*font.Face
	for _, res := range resources {
		f := loadMeasureFont(res)
		if f == nil {
			t.Fatalf("bundled font %s failed to load", res.Name())
		}
		faces = append(faces, f)
	}

	covered := func(r rune) bool {
		for _, f := range faces {
			if _, ok := f.NominalGlyph(r); ok {
				return true
			}
		}
		return false
	}

	// Checks, ballots and crosses; bullets; stars and suit/warning signs;
	// keyboard and macOS modifier symbols; arrows; geometric shapes;
	// letterlike symbols and fractions; math operators; misc punctuation;
	// enclosed alphanumerics.
	symbols := []rune{
		'✓', '✔', '✗', '✘', '✕', '☐', '☑', '☒',
		'•', '◦', '‣', '·',
		'★', '☆', '⚠', '♥', '♦', '♣', '♠',
		'⌘', '⌥', '⇧', '⌃', '⌫', '⌦', '⎋', '⏎', '⇥', '␣', '⏏',
		'←', '→', '↑', '↓', '↔', '↕', '⇐', '⇒', '↩', '↪',
		'▲', '▼', '◀', '▶', '■', '□', '▪', '▫', '●', '○', '◆', '◇', '◉',
		'№', '™', '½', '⅓', '¼', 'Ⅰ', 'Ⅱ',
		'≤', '≥', '≠', '≈', '±', '∞', '∑', '√', '∆', '∈', '∪', '∩',
		'※', '‽', '⁂', '⁃',
		'①', '②', '③', 'Ⓐ',
	}
	for _, r := range symbols {
		if !covered(r) {
			t.Errorf("rune %U %q is not covered by any compiled-in face — its metrics would be host-dependent", r, string(r))
		}
	}
}
