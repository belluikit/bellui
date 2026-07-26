// Package theme defines how a Bellui app should look when rendered.
package theme // import "github.com/belluikit/bellui/theme"

import (
	"image/color"
	"os"
	"strings"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
	internaltheme "github.com/belluikit/bellui/internal/theme"
)

// Keep in mind to add new constants to the tests at test/theme.go.
const (
	// VariantDark is the version of a theme that satisfies a user preference for a dark look.
	//
	// Since: 2.0
	VariantDark = internaltheme.VariantDark

	// VariantLight is the version of a theme that satisfies a user preference for a light look.
	//
	// Since: 2.0
	VariantLight = internaltheme.VariantLight
)

var defaultTheme, systemTheme bellui.Theme

// darkTheme returns the built-in dark theme, used only by tests that need a
// deterministic variant (exposed to test builds via export_test.go). It is not
// part of the public API: applications must not hard-code a variant, they should
// follow the user preference through DefaultTheme or set a custom theme.
func darkTheme() bellui.Theme {
	theme := &builtinTheme{variant: VariantDark}

	theme.initFonts()
	return theme
}

// DefaultTheme returns a built-in theme that can adapt to the user preference of light or dark colors.
//
// Since: 2.0
func DefaultTheme() bellui.Theme {
	if defaultTheme == nil {
		defaultTheme = setupDefaultTheme()
	}

	// check system too
	if systemTheme != nil {
		return systemTheme
	}

	return defaultTheme
}

// lightTheme returns the built-in light theme, used only by tests that need a
// deterministic variant (exposed to test builds via export_test.go). See
// darkTheme for why this is not part of the public API.
func lightTheme() bellui.Theme {
	theme := &builtinTheme{variant: VariantLight}

	theme.initFonts()
	return theme
}

type builtinTheme struct {
	variant bellui.ThemeVariant

	regular, bold, italic, boldItalic            bellui.Resource
	medium, mediumItalic                         bellui.Resource
	monospace, monoItalic, monoBold, monoBoldIta bellui.Resource
	symbol                                       bellui.Resource
}

func (t *builtinTheme) initFonts() {
	t.regular = regular
	t.bold = bold
	t.italic = italic
	t.boldItalic = bolditalic
	t.medium = medium
	t.mediumItalic = mediumItalic
	t.monospace = monospace
	t.monoItalic = monoItalicRes
	t.monoBold = monoBoldRes
	t.monoBoldIta = monoBoldItalicRes
	t.symbol = symbol

	font := os.Getenv("BELLUI_FONT")
	if font != "" {
		t.regular = loadCustomFont(font, "Regular", regular)
		if t.regular == regular { // failed to load
			t.bold = loadCustomFont(font, "Bold", bold)
			t.italic = loadCustomFont(font, "Italic", italic)
			t.boldItalic = loadCustomFont(font, "BoldItalic", bolditalic)
		} else { // first custom font loaded, fall back to that
			t.bold = loadCustomFont(font, "Bold", t.regular)
			t.italic = loadCustomFont(font, "Italic", t.regular)
			t.boldItalic = loadCustomFont(font, "BoldItalic", t.regular)
		}
	}
	font = os.Getenv("BELLUI_FONT_MONOSPACE")
	if font != "" {
		t.monospace = loadCustomFont(font, "Regular", monospace)
	}
	font = os.Getenv("BELLUI_FONT_SYMBOL")
	if font != "" {
		t.symbol = loadCustomFont(font, "Regular", symbol)
	}
}

func (t *builtinTheme) Color(n bellui.ThemeColorName, v bellui.ThemeVariant) color.Color {
	if t.variant != internaltheme.VariantNameUserPreference {
		v = t.variant
	}

	primary := bellui.CurrentApp().Settings().PrimaryColor()
	if n == ColorNamePrimary || n == ColorNameHyperlink {
		return internaltheme.PrimaryColorNamed(primary)
	} else if n == ColorNameForegroundOnPrimary {
		return internaltheme.ForegroundOnPrimaryColorNamed(primary)
	} else if n == ColorNameFocus {
		return focusColorNamed(primary)
	} else if n == ColorNameSelection {
		return selectionColorNamed(primary)
	}

	if v == VariantLight {
		return lightPaletteColorNamed(n)
	}

	return darkPaletteColorNamed(n)
}

func (t *builtinTheme) Font(style bellui.TextStyle) bellui.Resource {
	if style.Monospace {
		if style.Bold && style.Italic {
			return t.monoBoldIta
		}
		if style.Bold {
			return t.monoBold
		}
		if style.Italic {
			return t.monoItalic
		}
		return t.monospace
	}
	// Medium weight (500) — between regular and bold.
	if style.ResolvedWeight() == bellui.WeightMedium {
		if style.Italic {
			return t.mediumItalic
		}
		return t.medium
	}
	if style.Bold {
		if style.Italic {
			return t.boldItalic
		}
		return t.bold
	}
	if style.Italic {
		return t.italic
	}
	if style.Symbol {
		return t.symbol
	}
	return t.regular
}

// Current returns the theme that is currently used for the running application.
// It looks up based on user preferences and application configuration.
//
// Since: 2.5
func Current() bellui.Theme {
	app := bellui.CurrentApp()
	if app == nil {
		return darkTheme()
	}
	currentTheme := app.Settings().Theme()
	if currentTheme == nil {
		return darkTheme()
	}

	return internaltheme.CurrentlyRenderingWithFallback(currentTheme)
}

// CurrentForWidget returns the theme that is currently used for the specified widget.
// It looks for widget overrides and falls back to the application's current theme.
//
// Since: 2.5
func CurrentForWidget(w core.CanvasObject) bellui.Theme {
	if custom := cache.WidgetTheme(w); custom != nil {
		return custom
	}

	return Current()
}

func currentVariant() bellui.ThemeVariant {
	cur := Current()
	if std, ok := cur.(*builtinTheme); ok {
		if std.variant != internaltheme.VariantNameUserPreference {
			return std.variant // override if using the old LightTheme() or DarkTheme() constructor
		}
	}
	if tv, ok := cur.(*enhancedTheme); ok {
		if tv.builtin.variant != internaltheme.VariantNameUserPreference {
			return tv.builtin.variant
		}
	}

	return bellui.CurrentApp().Settings().ThemeVariant()
}

func darkPaletteColorNamed(name bellui.ThemeColorName) color.Color {
	switch name {
	case ColorNameBackground:
		return colorDarkBackground
	case ColorNameButton:
		return colorDarkButton
	case ColorNameDisabled:
		return colorDarkDisabled
	case ColorNameDisabledButton:
		return colorDarkDisabledButton
	case ColorNameError:
		return colorDarkError
	case ColorNameForeground:
		return colorDarkForeground
	case ColorNameForegroundOnError:
		return colorDarkForegroundOnError
	case ColorNameForegroundOnSuccess:
		return colorDarkForegroundOnSuccess
	case ColorNameForegroundOnWarning:
		return colorDarkForegroundOnWarning
	case ColorNameHover:
		return colorDarkHover
	case ColorNameHeaderBackground:
		return colorDarkHeaderBackground
	case ColorNameInputBackground:
		return colorDarkInputBackground
	case ColorNameInputBorder:
		return colorDarkInputBorder
	case ColorNameMenuBackground:
		return colorDarkMenuBackground
	case ColorNameOverlayBackground:
		return colorDarkOverlayBackground
	case ColorNamePlaceHolder:
		return colorDarkPlaceholder
	case ColorNamePressed:
		return colorDarkPressed
	case ColorNameScrollBar:
		return colorDarkScrollBar
	case ColorNameScrollBarBackground:
		return colorDarkScrollBarBackground
	case ColorNameSeparator:
		return colorDarkSeparator
	case ColorNameShadow:
		return colorDarkShadow
	case ColorNameSuccess:
		return colorDarkSuccess
	case ColorNameWarning:
		return colorDarkWarning
	}

	warnUnknownColor(name)
	return color.Transparent
}

func focusColorNamed(name string) color.NRGBA {
	switch name {
	case ColorRed:
		return colorLightFocusRed
	case ColorOrange:
		return colorLightFocusOrange
	case ColorYellow:
		return colorLightFocusYellow
	case ColorGreen:
		return colorLightFocusGreen
	case ColorPurple:
		return colorLightFocusPurple
	case ColorBrown:
		return colorLightFocusBrown
	case ColorGray:
		return colorLightFocusGray
	}

	// We return the value for ColorBlue for every other value.
	// There is no need to have it in the switch above.
	return colorLightFocusBlue
}

func lightPaletteColorNamed(name bellui.ThemeColorName) color.Color {
	switch name {
	case ColorNameBackground:
		return colorLightBackground
	case ColorNameButton:
		return colorLightButton
	case ColorNameDisabled:
		return colorLightDisabled
	case ColorNameDisabledButton:
		return colorLightDisabledButton
	case ColorNameError:
		return colorLightError
	case ColorNameForeground:
		return colorLightForeground
	case ColorNameForegroundOnError:
		return colorLightForegroundOnError
	case ColorNameForegroundOnSuccess:
		return colorLightForegroundOnSuccess
	case ColorNameForegroundOnWarning:
		return colorLightForegroundOnWarning
	case ColorNameHover:
		return colorLightHover
	case ColorNameHeaderBackground:
		return colorLightHeaderBackground
	case ColorNameInputBackground:
		return colorLightInputBackground
	case ColorNameInputBorder:
		return colorLightInputBorder
	case ColorNameMenuBackground:
		return colorLightMenuBackground
	case ColorNameOverlayBackground:
		return colorLightOverlayBackground
	case ColorNamePlaceHolder:
		return colorLightPlaceholder
	case ColorNamePressed:
		return colorLightPressed
	case ColorNameScrollBar:
		return colorLightScrollBar
	case ColorNameScrollBarBackground:
		return colorLightScrollBarBackground
	case ColorNameSeparator:
		return colorLightSeparator
	case ColorNameShadow:
		return colorLightShadow
	case ColorNameSuccess:
		return colorLightSuccess
	case ColorNameWarning:
		return colorLightWarning
	}

	warnUnknownColor(name)
	return color.Transparent
}

func loadCustomFont(env, variant string, fallback bellui.Resource) bellui.Resource {
	variantPath := strings.ReplaceAll(env, "Regular", variant)

	res, err := bellui.LoadResourceFromPath(variantPath)
	if err != nil {
		bellui.LogError("Error loading specified font", err)
		return fallback
	}

	return res
}

func selectionColorNamed(name string) color.NRGBA {
	switch name {
	case ColorRed:
		return colorLightSelectionRed
	case ColorOrange:
		return colorLightSelectionOrange
	case ColorYellow:
		return colorLightSelectionYellow
	case ColorGreen:
		return colorLightSelectionGreen
	case ColorPurple:
		return colorLightSelectionPurple
	case ColorBrown:
		return colorLightSelectionBrown
	case ColorGray:
		return colorLightSelectionGray
	}

	// We return the value for ColorBlue for every other value.
	// There is no need to have it in the switch above.
	return colorLightSelectionBlue
}

func setupDefaultTheme() bellui.Theme {
	builtin := &builtinTheme{variant: internaltheme.VariantNameUserPreference}
	builtin.initFonts()

	tv := &enhancedTheme{builtin: builtin}
	systemTheme = setupSystemTheme(tv)

	return tv
}
