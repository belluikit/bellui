package painter

import (
	"sync"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/typesetting/font"
)

func init() {
	bellui.SetFontFallbackRegister(registerFallback)
}

var (
	fallbackMu    sync.RWMutex
	fallbackFaces []*font.Face
)

// registerFallback loads res and appends it to the global fallback chain. It
// backs [bellui.RegisterFontFallback].
func registerFallback(res bellui.Resource) {
	face := loadMeasureFont(res)
	if face == nil {
		return
	}

	fallbackMu.Lock()
	fallbackFaces = append(fallbackFaces, face)
	fallbackMu.Unlock()

	// dynamicFontMaps consult the registry live, but cached measurements
	// (advances) for runes that were previously uncovered are now stale.
	ClearFontCache()
	cache.ResetThemeCaches()
}

// resolveFallback returns the first registered fallback face that covers r, or
// nil when none do.
func resolveFallback(r rune) *font.Face {
	fallbackMu.RLock()
	defer fallbackMu.RUnlock()

	for _, f := range fallbackFaces {
		if _, ok := f.NominalGlyph(r); ok {
			return f
		}
	}
	return nil
}
