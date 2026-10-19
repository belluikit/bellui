package painter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
)

// resetSystemFontScan forgets any system font scan an earlier test ran, so the
// test sees whether its own text triggers one, and restores it afterwards.
func resetSystemFontScan(t *testing.T) {
	t.Helper()
	fontScanLock.Lock()
	prevMap, prevLoaded := fm, loaded
	fm, loaded = nil, false
	fontScanLock.Unlock()
	ClearFontCache()
	t.Cleanup(func() {
		fontScanLock.Lock()
		fm, loaded = prevMap, prevLoaded
		fontScanLock.Unlock()
		ClearFontCache()
	})
}

func systemFontsScanned() bool {
	fontScanLock.Lock()
	defer fontScanLock.Unlock()
	return loaded
}

// TestCoveredTextDoesNotScanSystemFonts pins #469: text the bundled faces cover
// is measured without scanning the system fonts, which cost every app ~75 MB of
// startup allocation when the locale face was looked up eagerly.
func TestCoveredTextDoesNotScanSystemFonts(t *testing.T) {
	resetSystemFontScan(t)

	for _, style := range []bellui.TextStyle{{}, {Bold: true}, {Italic: true}, {Monospace: true}} {
		size, _ := RenderedTextSize("Hello, World 0123", 14, style, nil)
		require.Positive(t, size.Width, "style %+v measured nothing", style)
	}
	assert.False(t, systemFontsScanned(), "measuring covered text scanned the system fonts")
}

// TestUncoveredRuneLooksUpLocaleFace checks the deferred lookup still happens:
// a rune no bundled face covers reaches the host locale face and the system
// scan, in the place the eager lookup gave it.
func TestUncoveredRuneLooksUpLocaleFace(t *testing.T) {
	resetSystemFontScan(t)

	faces := CachedFontFace(bellui.TextStyle{}, nil, nil).Fonts.(*dynamicFontMap)
	bundled := len(faces.faces)
	require.True(t, faces.localePending)

	const uncovered = 'ꙮ' // CYRILLIC LETTER MULTIOCULAR O: in no bundled face
	for _, f := range faces.faces {
		_, ok := f.NominalGlyph(uncovered)
		require.False(t, ok, "a bundled face covers %U; pick another rune", uncovered)
	}
	faces.ResolveFace(uncovered)

	assert.True(t, systemFontsScanned(), "an uncovered rune did not reach the system fonts")
	assert.False(t, faces.localePending, "the locale face lookup is still pending")
	if faces.systemStart > 0 {
		assert.Equal(t, bundled, faces.systemStart, "the locale face must follow the bundled faces")
	}
}
