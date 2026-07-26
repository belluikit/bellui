package editor

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

// Syntax color name constants for use in [StyleRun.ColorName].
// These follow the semantic naming convention so themes can provide
// language-agnostic syntax coloring.
//
// If the application theme does not define these names, [themeColor]
// falls back to sensible defaults (see [defaultColors]).
const (
	// ColorNameSyntaxKeyword colors language keywords (if, for, return, etc.).
	ColorNameSyntaxKeyword bellui.ThemeColorName = "editor.syntax.keyword"

	// ColorNameSyntaxString colors string literals.
	ColorNameSyntaxString bellui.ThemeColorName = "editor.syntax.string"

	// ColorNameSyntaxComment colors comments.
	ColorNameSyntaxComment bellui.ThemeColorName = "editor.syntax.comment"

	// ColorNameSyntaxNumber colors numeric literals.
	ColorNameSyntaxNumber bellui.ThemeColorName = "editor.syntax.number"

	// ColorNameSyntaxBuiltin colors built-in functions and types.
	ColorNameSyntaxBuiltin bellui.ThemeColorName = "editor.syntax.builtin"

	// ColorNameSyntaxFunction colors function names.
	ColorNameSyntaxFunction bellui.ThemeColorName = "editor.syntax.function"

	// ColorNameSyntaxOperator colors operators and punctuation.
	ColorNameSyntaxOperator bellui.ThemeColorName = "editor.syntax.operator"

	// ColorNameSyntaxType colors type names.
	ColorNameSyntaxType bellui.ThemeColorName = "editor.syntax.type"

	// ColorNameGutterText colors line numbers and other gutter text.
	ColorNameGutterText bellui.ThemeColorName = "editor.gutter.text"

	// ColorNameGutterBackground colors the gutter background.
	ColorNameGutterBackground bellui.ThemeColorName = "editor.gutter.background"

	// ColorNameDiagnosticError colors error squiggles and gutter markers.
	ColorNameDiagnosticError bellui.ThemeColorName = "editor.diagnostic.error"

	// ColorNameDiagnosticWarning colors warning squiggles and gutter markers.
	ColorNameDiagnosticWarning bellui.ThemeColorName = "editor.diagnostic.warning"

	// ColorNameDiagnosticInfo colors informational squiggles and gutter markers.
	ColorNameDiagnosticInfo bellui.ThemeColorName = "editor.diagnostic.info"

	// ColorNameDiagnosticHint colors hint squiggles and gutter markers.
	ColorNameDiagnosticHint bellui.ThemeColorName = "editor.diagnostic.hint"

	// ColorNameRedaction colors the opaque masking bar drawn over a redacted
	// (masked) secret span. The bar is painted fully opaque regardless of the
	// color's alpha, so only its RGB is meaningful.
	ColorNameRedaction bellui.ThemeColorName = "editor.redaction"

	// ColorNameFoldMarker colors the fold/unfold chevrons drawn in the fold
	// gutter on foldable lines.
	ColorNameFoldMarker bellui.ThemeColorName = "editor.fold.marker"

	// ColorNameFoldPlaceholder colors the inline ellipsis/summary placeholder
	// shown after a collapsed region's header line.
	ColorNameFoldPlaceholder bellui.ThemeColorName = "editor.fold.placeholder"
)

// defaultDarkColors provides fallback syntax colors for dark backgrounds (Monokai-inspired).
var defaultDarkColors = map[bellui.ThemeColorName]color.Color{
	ColorNameSyntaxKeyword:    color.NRGBA{R: 0xF9, G: 0x26, B: 0x72, A: 0xFF}, // pink
	ColorNameSyntaxString:     color.NRGBA{R: 0xE6, G: 0xDB, B: 0x74, A: 0xFF}, // yellow
	ColorNameSyntaxComment:    color.NRGBA{R: 0x88, G: 0x84, B: 0x6F, A: 0xFF}, // warm grey
	ColorNameSyntaxNumber:     color.NRGBA{R: 0xAE, G: 0x81, B: 0xFF, A: 0xFF}, // purple
	ColorNameSyntaxBuiltin:    color.NRGBA{R: 0x66, G: 0xD9, B: 0xEF, A: 0xFF}, // cyan
	ColorNameSyntaxFunction:   color.NRGBA{R: 0xA6, G: 0xE2, B: 0x2E, A: 0xFF}, // bright green
	ColorNameSyntaxOperator:   color.NRGBA{R: 0xD0, G: 0xD0, B: 0xD0, A: 0xFF}, // light grey
	ColorNameSyntaxType:       color.NRGBA{R: 0x66, G: 0xD9, B: 0xEF, A: 0xFF}, // cyan
	ColorNameGutterText:       color.NRGBA{R: 0x90, G: 0x90, B: 0x90, A: 0xFF}, // grey
	ColorNameGutterBackground: color.Transparent,

	ColorNameDiagnosticError:   color.NRGBA{R: 0xF4, G: 0x47, B: 0x47, A: 0xFF}, // red
	ColorNameDiagnosticWarning: color.NRGBA{R: 0xE2, G: 0xB9, B: 0x3B, A: 0xFF}, // amber
	ColorNameDiagnosticInfo:    color.NRGBA{R: 0x4F, G: 0xA6, B: 0xF5, A: 0xFF}, // blue
	ColorNameDiagnosticHint:    color.NRGBA{R: 0x9A, G: 0x9A, B: 0x9A, A: 0xFF}, // grey

	ColorNameRedaction: color.NRGBA{R: 0x2A, G: 0x2A, B: 0x2E, A: 0xFF}, // near-black bar

	ColorNameFoldMarker:      color.NRGBA{R: 0x90, G: 0x90, B: 0x90, A: 0xFF}, // grey chevron
	ColorNameFoldPlaceholder: color.NRGBA{R: 0x88, G: 0x84, B: 0x6F, A: 0xFF}, // comment warm grey
}

// defaultLightColors provides high-contrast syntax colors for light backgrounds.
var defaultLightColors = map[bellui.ThemeColorName]color.Color{
	ColorNameSyntaxKeyword:    color.NRGBA{R: 0xD7, G: 0x00, B: 0x40, A: 0xFF}, // crimson
	ColorNameSyntaxString:     color.NRGBA{R: 0x03, G: 0x2F, B: 0x62, A: 0xFF}, // dark blue
	ColorNameSyntaxComment:    color.NRGBA{R: 0x6A, G: 0x73, B: 0x7D, A: 0xFF}, // slate grey
	ColorNameSyntaxNumber:     color.NRGBA{R: 0x00, G: 0x5C, B: 0xC5, A: 0xFF}, // blue
	ColorNameSyntaxBuiltin:    color.NRGBA{R: 0x00, G: 0x6B, B: 0x75, A: 0xFF}, // teal
	ColorNameSyntaxFunction:   color.NRGBA{R: 0x6F, G: 0x42, B: 0x00, A: 0xFF}, // dark amber
	ColorNameSyntaxOperator:   color.NRGBA{R: 0x24, G: 0x29, B: 0x2E, A: 0xFF}, // near-black
	ColorNameSyntaxType:       color.NRGBA{R: 0x00, G: 0x6B, B: 0x75, A: 0xFF}, // teal
	ColorNameGutterText:       color.NRGBA{R: 0x6E, G: 0x76, B: 0x81, A: 0xFF}, // grey
	ColorNameGutterBackground: color.Transparent,

	ColorNameDiagnosticError:   color.NRGBA{R: 0xD7, G: 0x2A, B: 0x2A, A: 0xFF}, // red
	ColorNameDiagnosticWarning: color.NRGBA{R: 0xA6, G: 0x7C, B: 0x00, A: 0xFF}, // dark amber
	ColorNameDiagnosticInfo:    color.NRGBA{R: 0x00, G: 0x5C, B: 0xC5, A: 0xFF}, // blue
	ColorNameDiagnosticHint:    color.NRGBA{R: 0x6A, G: 0x73, B: 0x7D, A: 0xFF}, // slate grey

	ColorNameRedaction: color.NRGBA{R: 0x33, G: 0x33, B: 0x37, A: 0xFF}, // dark bar

	ColorNameFoldMarker:      color.NRGBA{R: 0x6E, G: 0x76, B: 0x81, A: 0xFF}, // grey chevron
	ColorNameFoldPlaceholder: color.NRGBA{R: 0x6A, G: 0x73, B: 0x7D, A: 0xFF}, // slate grey
}

// defaultColors returns the variant-appropriate fallback color map.
func defaultColors() map[bellui.ThemeColorName]color.Color {
	if a := bellui.CurrentApp(); a != nil {
		if a.Settings().ThemeVariant() == theme.VariantLight {
			return defaultLightColors
		}
	}
	return defaultDarkColors
}

// themeColor resolves a named color from the current application theme.
// If the theme does not define the name, falls back to [defaultColors].
// If no application is running (e.g. in tests), returns the default or
// the theme foreground color.
func themeColor(name bellui.ThemeColorName) color.Color {
	// Always prefer our own variant-aware defaults for fycoder-specific names.
	// The app theme's fallback for unknown names is unreliable (Fyne returns
	// a catch-all color like blue instead of transparent).
	defaults := defaultColors()
	if def, ok := defaults[name]; ok {
		return def
	}
	// For non-fycoder names (e.g. theme.ColorNameForeground), use the app theme.
	if a := bellui.CurrentApp(); a != nil {
		return a.Settings().Theme().Color(name, a.Settings().ThemeVariant())
	}
	return color.White
}
