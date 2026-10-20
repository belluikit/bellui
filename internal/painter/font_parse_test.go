package painter

import (
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
		assert.Contains(t, chain.builtFrom, emoji, "style %+v has its own parse of the emoji font", styles[i])
	}
}

// TestSweepReleasesUnusedParsedFonts checks parsed fonts live as long as a
// cached chain uses them, so an idle app still releases them.
func TestSweepReleasesUnusedParsedFonts(t *testing.T) {
	ClearFontCache()
	t.Cleanup(ClearFontCache)

	CachedFontFace(bellui.TextStyle{}, nil, nil)
	SweepFontCache(time.Hour)
	parses := FontParses()
	loadMeasureFont(theme.DefaultEmojiFont())
	assert.Equal(t, parses, FontParses(), "a sweep released a font a cached chain uses")

	SweepFontCache(-time.Hour) // expires every chain
	loadMeasureFont(theme.DefaultEmojiFont())
	assert.Equal(t, parses+1, FontParses(), "a sweep kept a font no cached chain uses")
}
