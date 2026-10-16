package painter

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
	"github.com/belluikit/typesetting/font"
)

//go:embed testdata/NotoCJKtest.otf
var testCJKFont []byte

func testCJKResource() bellui.Resource {
	return &bellui.StaticResource{StaticName: "NotoCJKtest.otf", StaticContent: testCJKFont}
}

func resetFallbacks() {
	fallbackMu.Lock()
	fallbackFaces = nil
	fallbackMu.Unlock()
}

func TestFontFallback_CoversUnknownRune(t *testing.T) {
	resetFallbacks()
	t.Cleanup(resetFallbacks)

	cjk := loadMeasureFont(testCJKResource())
	require.NotNil(t, cjk, "tiny CJK test font should parse")
	_, ok := cjk.NominalGlyph('世')
	require.True(t, ok, "test font should cover 世")

	latin := loadMeasureFont(theme.DefaultTextFont())
	require.NotNil(t, latin)
	if _, ok := latin.NominalGlyph('世'); ok {
		t.Skip("default font unexpectedly covers CJK")
	}

	assert.Nil(t, resolveFallback('世'), "no fallback before registration")

	registerFallback(testCJKResource())
	assert.NotNil(t, resolveFallback('世'), "registered fallback covers 世")
	assert.Nil(t, resolveFallback('🙂'), "rune outside the CJK subset stays uncovered")

	// A Latin-only map resolves the CJK rune to the registered fallback face,
	// rather than a tofu/.notdef from faces[0].
	d := &dynamicFontMap{family: "test", faces: []*font.Face{latin}}
	got := d.ResolveFace('世')
	_, ok = got.NominalGlyph('世')
	assert.True(t, ok, "ResolveFace returns a covering face for the CJK rune")
}

// TestMeasure_CJKFallbackGrowsLineBox locks in the pre-rebrand tracker #130 follow-up fix: the line
// box measured for text is sized from every run's face, not just the primary
// Latin one. A taller fallback face (the bundled CJK font, pre-rebrand tracker #314) must grow the
// line height and baseline so its glyphs are not clipped at the bottom of an
// Entry. Latin-only text, which shapes in the same face as the measured space,
// is unaffected.
func TestMeasure_CJKFallbackGrowsLineBox(t *testing.T) {
	resetFallbacks()
	t.Cleanup(resetFallbacks)
	ClearFontCache()
	t.Cleanup(ClearFontCache)

	const size = 20
	const latin = "Hello"
	const cjk = "世界" // covered by the test fallback font, not the Latin default

	latinBefore, _ := RenderedTextSize(latin, size, bellui.TextStyle{}, nil)

	// Register the tall CJK face so a covering face exists on every platform
	// (headless CI has no system CJK fonts to scan). Consulted before the system
	// scan, so 世界 resolves to it.
	registerFallback(testCJKResource()) // clears the measure cache internally

	latinAfter, _ := RenderedTextSize(latin, size, bellui.TextStyle{}, nil)
	cjkAfter, baseline := RenderedTextSize(cjk, size, bellui.TextStyle{}, nil)

	// The fix: a taller run grows the line box. Under the old primary-face-only
	// measurement, cjkAfter.Height would equal latinAfter.Height (the space's
	// Latin metrics) and CJK glyphs clipped at the bottom of an Entry.
	assert.Equal(t, latinBefore.Height, latinAfter.Height,
		"Latin line height is unaffected by a registered fallback")
	assert.Greater(t, cjkAfter.Height, latinAfter.Height,
		"CJK line box grows to fit the taller fallback face")
	assert.Greater(t, baseline, float32(0), "baseline (ascent) is positive")
}

func TestRegisterFontFallback_PublicHookWired(t *testing.T) {
	resetFallbacks()
	t.Cleanup(resetFallbacks)

	// internal/painter.init() wired bellui.RegisterFontFallback to registerFallback.
	bellui.RegisterFontFallback(testCJKResource())
	assert.NotNil(t, resolveFallback('界'), "bellui.RegisterFontFallback reaches the painter")

	bellui.RegisterFontFallback(nil) // no-op, must not panic
}
