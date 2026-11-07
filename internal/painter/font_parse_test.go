package painter

import (
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

// TestFontChainsShareParsedFonts pins #446: the chain of every text style
// includes the bundled emoji font, which was parsed again for each of them.
func TestFontChainsShareParsedFonts(t *testing.T) {
	ClearFontCache()
	t.Cleanup(ClearFontCache)

	styles := []bellui.TextStyle{{}, {Bold: true}, {Italic: true}, {Bold: true, Italic: true}, {Monospace: true}}
	var chains []*FontCacheItem
	for _, style := range styles {
		chains = append(chains, CachedFontFace(style, nil, nil))
	}

	parses := FontParses()
	emoji := loadMeasureFont(theme.DefaultEmojiFont())
	require.NotNil(t, emoji)
	assert.Equal(t, parses, FontParses(), "the emoji font was parsed again")
	for i, chain := range chains {
		assert.Contains(t, chain.Fonts.(*dynamicFontMap).faces, emoji, "style %+v has its own parse of the emoji font", styles[i])
	}
}

// TestSweepReleasesUnusedParsedFonts checks parsed fonts live as long as
// something uses them, so an idle app still releases them.
func TestSweepReleasesUnusedParsedFonts(t *testing.T) {
	ClearFontCache()
	t.Cleanup(ClearFontCache)

	CachedFontFace(bellui.TextStyle{}, nil, nil)
	SweepFontCache(time.Hour)
	runtime.GC()
	parses := FontParses()
	loadMeasureFont(theme.DefaultEmojiFont())
	assert.Equal(t, parses, FontParses(), "a sweep released a font a cached chain uses")

	SweepFontCache(-time.Hour) // expires every chain
	runtime.GC()
	loadMeasureFont(theme.DefaultEmojiFont())
	assert.Equal(t, parses+1, FontParses(), "a font nothing uses was kept")

	held := loadMeasureFont(theme.DefaultEmojiFont())
	SweepFontCache(-time.Hour)
	runtime.GC()
	assert.Same(t, held, loadMeasureFont(theme.DefaultEmojiFont()), "a font still held was parsed again (#1066)")
	assert.Equal(t, parses+1, FontParses(), "a font still held was parsed again (#1066)")
	SweepFontCache(-time.Hour)
	assert.Equal(t, 1, parsedFonts.Len(), "the sweep forgot a font still held")
	runtime.KeepAlive(held)
}
