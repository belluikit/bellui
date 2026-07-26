//go:build !no_symbols

package theme

import (
	_ "embed"

	"github.com/belluikit/bellui"
)

// Bundled UI-symbol fallback faces (#686): subsets of Noto Sans Symbols 2 and
// Noto Sans Symbols (SIL OFL 1.1, see font/LICENSE_NotoSansSymbols.txt)
// covering the common symbol blocks the primary bundled fonts lack — General
// Punctuation gaps, Letterlike Symbols, Number Forms, Arrows, Mathematical
// Operators, Miscellaneous Technical (⌘ ⌥ ⇧), Control Pictures, Enclosed
// Alphanumerics, Geometric Shapes, Miscellaneous Symbols (☐ ★), Dingbats
// (✓ ✗) and Miscellaneous Symbols and Arrows.
//
// Without these, such glyphs fall through to the host system-font scan, so
// their metrics differ per OS (host-dependent layout, tofu on wasm). The
// subsets are consulted after the theme, fallback and emoji faces but before
// any host font, keeping their metrics identical on every platform.
//
// Build with -tags no_symbols to strip them (~240 KB) from a binary.

//go:embed font/NotoSansSymbols2-bellui.ttf
var symbolsFallback2Data []byte

//go:embed font/NotoSansSymbols-bellui.ttf
var symbolsFallbackData []byte

//go:embed font/NotoSansPunct-bellui.ttf
var symbolsFallbackPunctData []byte

//go:embed font/NotoSansMath-bellui.ttf
var symbolsFallbackMathData []byte

var symbolsFallbackFonts = []bellui.Resource{
	&bellui.StaticResource{
		StaticName:    "NotoSansSymbols2-bellui.ttf",
		StaticContent: symbolsFallback2Data,
	},
	&bellui.StaticResource{
		StaticName:    "NotoSansSymbols-bellui.ttf",
		StaticContent: symbolsFallbackData,
	},
	// General Punctuation and other block gaps neither Symbols face covers
	// (‣ ※ ‽, zero-width joiners/marks), from the OFL Noto Sans already in
	// font/ (see font/LICENSE.txt).
	&bellui.StaticResource{
		StaticName:    "NotoSansPunct-bellui.ttf",
		StaticContent: symbolsFallbackPunctData,
	},
	// Mathematical operators, double arrows and technical symbols (⇥ ⇒ ∈ ∪)
	// the faces above still lack, from OFL Noto Sans Math.
	&bellui.StaticResource{
		StaticName:    "NotoSansMath-bellui.ttf",
		StaticContent: symbolsFallbackMathData,
	},
}
