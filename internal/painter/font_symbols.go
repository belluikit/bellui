package painter

import (
	"sync"

	"github.com/belluikit/typesetting/font"

	"github.com/belluikit/bellui/theme"
)

var (
	symbolsFacesOnce sync.Once
	symbolsFaces     []*font.Face
)

// symbolsFallbackFaces returns the bundled UI-symbol fallback faces from
// [theme.DefaultSymbolsFallbackFonts], parsed once and shared by every font
// chain. They are part of the compiled-in face set — appended after the theme
// and emoji faces but before the host locale font — so common symbols (✓, ☐,
// arrows, ⌘, geometric shapes, …) measure and render identically on every
// platform instead of resolving through a host system font (#686).
func symbolsFallbackFaces() []*font.Face {
	symbolsFacesOnce.Do(func() {
		for _, res := range theme.DefaultSymbolsFallbackFonts() {
			if f := loadMeasureFont(res); f != nil {
				symbolsFaces = append(symbolsFaces, f)
			}
		}
	})
	return symbolsFaces
}

// addSymbolsFallbackFaces appends the bundled UI-symbol fallback faces to a
// font chain.
func addSymbolsFallbackFaces(faces *dynamicFontMap) {
	for _, f := range symbolsFallbackFaces() {
		faces.addFace(f)
	}
}
