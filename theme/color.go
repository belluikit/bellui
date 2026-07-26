package theme

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	internaltheme "github.com/belluikit/bellui/internal/theme"
)

// Keep in mind to add new constants to the tests at test/theme.go.
const (
	// ColorRed is the red primary color name.
	//
	// Since: 1.4
	ColorRed = internaltheme.ColorRed
	// ColorOrange is the orange primary color name.
	//
	// Since: 1.4
	ColorOrange = internaltheme.ColorOrange
	// ColorYellow is the yellow primary color name.
	//
	// Since: 1.4
	ColorYellow = internaltheme.ColorYellow
	// ColorGreen is the green primary color name.
	//
	// Since: 1.4
	ColorGreen = internaltheme.ColorGreen
	// ColorBlue is the blue primary color name.
	//
	// Since: 1.4
	ColorBlue = internaltheme.ColorBlue
	// ColorPurple is the purple primary color name.
	//
	// Since: 1.4
	ColorPurple = internaltheme.ColorPurple
	// ColorBrown is the brown primary color name.
	//
	// Since: 1.4
	ColorBrown = internaltheme.ColorBrown
	// ColorGray is the gray primary color name.
	//
	// Since: 1.4
	ColorGray = internaltheme.ColorGray

	// ColorNameBackground is the name of theme lookup for background color.
	//
	// Since: 2.0
	ColorNameBackground bellui.ThemeColorName = "background"

	// ColorNameButton is the name of theme lookup for button color.
	//
	// Since: 2.0
	ColorNameButton bellui.ThemeColorName = "button"

	// ColorNameDisabledButton is the name of theme lookup for disabled button color.
	//
	// Since: 2.0
	ColorNameDisabledButton bellui.ThemeColorName = "disabledButton"

	// ColorNameDisabled is the name of theme lookup for disabled foreground color.
	//
	// Since: 2.0
	ColorNameDisabled bellui.ThemeColorName = "disabled"

	// ColorNameError is the name of theme lookup for error color.
	//
	// Since: 2.0
	ColorNameError bellui.ThemeColorName = "error"

	// ColorNameFocus is the name of theme lookup for focus color.
	//
	// Since: 2.0
	ColorNameFocus bellui.ThemeColorName = "focus"

	// ColorNameForeground is the name of theme lookup for foreground color.
	//
	// Since: 2.0
	ColorNameForeground bellui.ThemeColorName = "foreground"

	// ColorNameForegroundOnError is the name of theme lookup for a contrast color to the error color.
	//
	// Since: 2.5
	ColorNameForegroundOnError bellui.ThemeColorName = "foregroundOnError"

	// ColorNameForegroundOnPrimary is the name of theme lookup for a contrast color to the primary color.
	//
	// Since: 2.5
	ColorNameForegroundOnPrimary bellui.ThemeColorName = "foregroundOnPrimary"

	// ColorNameForegroundOnSuccess is the name of theme lookup for a contrast color to the success color.
	//
	// Since: 2.5
	ColorNameForegroundOnSuccess bellui.ThemeColorName = "foregroundOnSuccess"

	// ColorNameForegroundOnWarning is the name of theme lookup for a contrast color to the warning color.
	//
	// Since: 2.5
	ColorNameForegroundOnWarning bellui.ThemeColorName = "foregroundOnWarning"

	// ColorNameHeaderBackground is the name of theme lookup for background color of a collection header.
	//
	// Since: 2.4
	ColorNameHeaderBackground bellui.ThemeColorName = "headerBackground"

	// ColorNameHover is the name of theme lookup for hover color.
	//
	// Since: 2.0
	ColorNameHover bellui.ThemeColorName = "hover"

	// ColorNameHyperlink is the name of theme lookup for hyperlink color.
	//
	// Since: 2.4
	ColorNameHyperlink bellui.ThemeColorName = "hyperlink"

	// ColorNameInputBackground is the name of theme lookup for background color of an input field.
	//
	// Since: 2.0
	ColorNameInputBackground bellui.ThemeColorName = "inputBackground"

	// ColorNameInputBorder is the name of theme lookup for border color of an input field.
	//
	// Since: 2.3
	ColorNameInputBorder bellui.ThemeColorName = "inputBorder"

	// ColorNameMenuBackground is the name of theme lookup for background color of menus.
	//
	// Since: 2.3
	ColorNameMenuBackground bellui.ThemeColorName = "menuBackground"

	// ColorNameOverlayBackground is the name of theme lookup for background color of overlays like dialogs.
	//
	// Since: 2.3
	ColorNameOverlayBackground bellui.ThemeColorName = "overlayBackground"

	// ColorNamePlaceHolder is the name of theme lookup for placeholder text color.
	//
	// Since: 2.0
	ColorNamePlaceHolder bellui.ThemeColorName = "placeholder"

	// ColorNamePressed is the name of theme lookup for the tap overlay color.
	//
	// Since: 2.0
	ColorNamePressed bellui.ThemeColorName = "pressed"

	// ColorNamePrimary is the name of theme lookup for primary color.
	//
	// Since: 2.0
	ColorNamePrimary bellui.ThemeColorName = "primary"

	// ColorNameScrollBar is the name of theme lookup for scrollbar color.
	//
	// Since: 2.0
	ColorNameScrollBar bellui.ThemeColorName = "scrollBar"

	// ColorNameScrollBarBackground is the name of theme lookup for scrollbar background color.
	//
	// Since: 2.6
	ColorNameScrollBarBackground bellui.ThemeColorName = "scrollBarBackground"

	// ColorNameSelection is the name of theme lookup for selection color.
	//
	// Since: 2.1
	ColorNameSelection bellui.ThemeColorName = "selection"

	// ColorNameSeparator is the name of theme lookup for separator bars.
	//
	// Since: 2.3
	ColorNameSeparator bellui.ThemeColorName = "separator"

	// ColorNameShadow is the name of theme lookup for shadow color.
	//
	// Since: 2.0
	ColorNameShadow bellui.ThemeColorName = "shadow"

	// ColorNameSuccess is the name of theme lookup for success color.
	//
	// Since: 2.3
	ColorNameSuccess bellui.ThemeColorName = "success"

	// ColorNameWarning is the name of theme lookup for warning color.
	//
	// Since: 2.3
	ColorNameWarning bellui.ThemeColorName = "warning"

	// ColorNameCardBackground is the name of theme lookup for card surface background color.
	// If a theme returns color.Transparent, cards will have no distinct background fill.
	ColorNameCardBackground bellui.ThemeColorName = "cardBackground"

	// ColorNameElevatedBackground is the name of theme lookup for elevated surface background color.
	// Used for surfaces that sit above the main background (panels, sidebars).
	ColorNameElevatedBackground bellui.ThemeColorName = "elevatedBackground"

	// --- MD3 Color Roles ---

	// ColorNamePrimaryContainer is the MD3 primary container color.
	ColorNamePrimaryContainer bellui.ThemeColorName = "primaryContainer"
	// ColorNameOnPrimaryContainer is text/icon color on primary container.
	ColorNameOnPrimaryContainer bellui.ThemeColorName = "onPrimaryContainer"

	// ColorNameSecondary is the MD3 secondary accent color.
	ColorNameSecondary bellui.ThemeColorName = "secondary"
	// ColorNameOnSecondary is text/icon color on secondary.
	ColorNameOnSecondary bellui.ThemeColorName = "onSecondary"
	// ColorNameSecondaryContainer is the MD3 secondary container color.
	ColorNameSecondaryContainer bellui.ThemeColorName = "secondaryContainer"
	// ColorNameOnSecondaryContainer is text/icon color on secondary container.
	ColorNameOnSecondaryContainer bellui.ThemeColorName = "onSecondaryContainer"

	// ColorNameTertiary is the MD3 tertiary accent color.
	ColorNameTertiary bellui.ThemeColorName = "tertiary"
	// ColorNameOnTertiary is text/icon color on tertiary.
	ColorNameOnTertiary bellui.ThemeColorName = "onTertiary"
	// ColorNameTertiaryContainer is the MD3 tertiary container color.
	ColorNameTertiaryContainer bellui.ThemeColorName = "tertiaryContainer"
	// ColorNameOnTertiaryContainer is text/icon color on tertiary container.
	ColorNameOnTertiaryContainer bellui.ThemeColorName = "onTertiaryContainer"

	// ColorNameErrorContainer is the MD3 error container color.
	ColorNameErrorContainer bellui.ThemeColorName = "errorContainer"
	// ColorNameOnErrorContainer is text/icon color on error container.
	ColorNameOnErrorContainer bellui.ThemeColorName = "onErrorContainer"

	// ColorNameSurface is the MD3 base surface color.
	ColorNameSurface bellui.ThemeColorName = "surface"
	// ColorNameSurfaceDim is the MD3 dimmed surface color.
	ColorNameSurfaceDim bellui.ThemeColorName = "surfaceDim"
	// ColorNameSurfaceBright is the MD3 bright surface color.
	ColorNameSurfaceBright bellui.ThemeColorName = "surfaceBright"
	// ColorNameSurfaceContainerLowest is the MD3 lowest-elevation surface container.
	ColorNameSurfaceContainerLowest bellui.ThemeColorName = "surfaceContainerLowest"
	// ColorNameSurfaceContainerLow is the MD3 low-elevation surface container.
	ColorNameSurfaceContainerLow bellui.ThemeColorName = "surfaceContainerLow"
	// ColorNameSurfaceContainerMedium is the MD3 medium-elevation surface container.
	ColorNameSurfaceContainerMedium bellui.ThemeColorName = "surfaceContainer"
	// ColorNameSurfaceContainerHigh is the MD3 high-elevation surface container.
	ColorNameSurfaceContainerHigh bellui.ThemeColorName = "surfaceContainerHigh"
	// ColorNameSurfaceContainerHighest is the MD3 highest-elevation surface container.
	ColorNameSurfaceContainerHighest bellui.ThemeColorName = "surfaceContainerHighest"
	// ColorNameOnSurface is text/icon color on surfaces.
	ColorNameOnSurface bellui.ThemeColorName = "onSurface"
	// ColorNameOnSurfaceVariant is secondary text/icon color on surfaces.
	ColorNameOnSurfaceVariant bellui.ThemeColorName = "onSurfaceVariant"

	// ColorNameOutline is the MD3 outline/border color.
	ColorNameOutline bellui.ThemeColorName = "outline"
	// ColorNameOutlineVariant is the MD3 subtle outline color.
	ColorNameOutlineVariant bellui.ThemeColorName = "outlineVariant"

	// ColorNameInverseSurface is the MD3 inverse surface color.
	ColorNameInverseSurface bellui.ThemeColorName = "inverseSurface"
	// ColorNameInverseOnSurface is text/icon color on inverse surface.
	ColorNameInverseOnSurface bellui.ThemeColorName = "inverseOnSurface"
	// ColorNameInversePrimary is the MD3 inverse primary color.
	ColorNameInversePrimary bellui.ThemeColorName = "inversePrimary"

	// ColorNameScrim is the MD3 scrim overlay color for modals.
	ColorNameScrim bellui.ThemeColorName = "scrim"
)

var (
	colorDarkBackground          = color.NRGBA{R: 0x17, G: 0x17, B: 0x18, A: 0xff}
	colorDarkButton              = color.NRGBA{R: 0x28, G: 0x29, B: 0x2e, A: 0xff}
	colorDarkDisabled            = color.NRGBA{R: 0x39, G: 0x39, B: 0x3a, A: 0xff}
	colorDarkDisabledButton      = color.NRGBA{R: 0x28, G: 0x29, B: 0x2e, A: 0xff}
	colorDarkError               = color.NRGBA{R: 0xf4, G: 0x43, B: 0x36, A: 0xff}
	colorDarkForeground          = color.NRGBA{R: 0xf3, G: 0xf3, B: 0xf3, A: 0xff}
	colorDarkForegroundOnError   = color.NRGBA{R: 0x17, G: 0x17, B: 0x18, A: 0xff}
	colorDarkForegroundOnSuccess = color.NRGBA{R: 0x17, G: 0x17, B: 0x18, A: 0xff}
	colorDarkForegroundOnWarning = color.NRGBA{R: 0x17, G: 0x17, B: 0x18, A: 0xff}
	colorDarkHeaderBackground    = color.NRGBA{R: 0x1b, G: 0x1b, B: 0x1b, A: 0xff}
	colorDarkHover               = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x0f}
	colorDarkInputBackground     = color.NRGBA{R: 0x20, G: 0x20, B: 0x23, A: 0xff}
	colorDarkInputBorder         = color.NRGBA{R: 0x39, G: 0x39, B: 0x3a, A: 0xff}
	colorDarkMenuBackground      = color.NRGBA{R: 0x28, G: 0x29, B: 0x2e, A: 0xff}
	colorDarkOverlayBackground   = color.NRGBA{R: 0x18, G: 0x1d, B: 0x25, A: 0xff}
	colorDarkPlaceholder         = color.NRGBA{R: 0xb2, G: 0xb2, B: 0xb2, A: 0xff}
	colorDarkPressed             = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x66}
	colorDarkScrollBar           = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0x99}
	colorDarkScrollBarBackground = color.NRGBA{R: 0x20, G: 0x20, B: 0x23, A: 0xff}
	colorDarkSeparator           = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}
	colorDarkShadow              = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x66}
	colorDarkSuccess             = color.NRGBA{R: 0x43, G: 0xf4, B: 0x36, A: 0xff}
	colorDarkWarning             = color.NRGBA{R: 0xff, G: 0x98, B: 0x00, A: 0xff}

	colorLightBackground          = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colorLightButton              = color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf5, A: 0xff}
	colorLightDisabled            = color.NRGBA{R: 0xe3, G: 0xe3, B: 0xe3, A: 0xff}
	colorLightDisabledButton      = color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf5, A: 0xff}
	colorLightError               = color.NRGBA{R: 0xf4, G: 0x43, B: 0x36, A: 0xff}
	colorLightFocusBlue           = color.NRGBA{R: 0x00, G: 0x6c, B: 0xff, A: 0x2a}
	colorLightFocusBrown          = color.NRGBA{R: 0x79, G: 0x55, B: 0x48, A: 0x7f}
	colorLightFocusGray           = color.NRGBA{R: 0x9e, G: 0x9e, B: 0x9e, A: 0x7f}
	colorLightFocusGreen          = color.NRGBA{R: 0x8b, G: 0xc3, B: 0x4a, A: 0x7f}
	colorLightFocusOrange         = color.NRGBA{R: 0xff, G: 0x98, B: 0x00, A: 0x7f}
	colorLightFocusPurple         = color.NRGBA{R: 0x9c, G: 0x27, B: 0xb0, A: 0x7f}
	colorLightFocusRed            = color.NRGBA{R: 0xf4, G: 0x43, B: 0x36, A: 0x7f}
	colorLightFocusYellow         = color.NRGBA{R: 0xff, G: 0xeb, B: 0x3b, A: 0x7f}
	colorLightForeground          = color.NRGBA{R: 0x56, G: 0x56, B: 0x56, A: 0xff}
	colorLightForegroundOnError   = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colorLightForegroundOnSuccess = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colorLightForegroundOnWarning = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colorLightHeaderBackground    = color.NRGBA{R: 0xf9, G: 0xf9, B: 0xf9, A: 0xff}
	colorLightHover               = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x0f}
	colorLightInputBackground     = color.NRGBA{R: 0xf3, G: 0xf3, B: 0xf3, A: 0xff}
	colorLightInputBorder         = color.NRGBA{R: 0xe3, G: 0xe3, B: 0xe3, A: 0xff}
	colorLightMenuBackground      = color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf5, A: 0xff}
	colorLightOverlayBackground   = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	colorLightPlaceholder         = color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xff}
	colorLightPressed             = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x19}
	colorLightScrollBar           = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x99}
	colorLightScrollBarBackground = color.NRGBA{R: 0xdb, G: 0xdb, B: 0xdb, A: 0xff}
	colorLightSelectionBlue       = color.NRGBA{R: 0x00, G: 0x6c, B: 0xff, A: 0x40}
	colorLightSelectionBrown      = color.NRGBA{R: 0x79, G: 0x55, B: 0x48, A: 0x3f}
	colorLightSelectionGray       = color.NRGBA{R: 0x9e, G: 0x9e, B: 0x9e, A: 0x3f}
	colorLightSelectionGreen      = color.NRGBA{R: 0x8b, G: 0xc3, B: 0x4a, A: 0x3f}
	colorLightSelectionOrange     = color.NRGBA{R: 0xff, G: 0x98, B: 0x00, A: 0x3f}
	colorLightSelectionPurple     = color.NRGBA{R: 0x9c, G: 0x27, B: 0xb0, A: 0x3f}
	colorLightSelectionRed        = color.NRGBA{R: 0xf4, G: 0x43, B: 0x36, A: 0x3f}
	colorLightSelectionYellow     = color.NRGBA{R: 0xff, G: 0xeb, B: 0x3b, A: 0x3f}
	colorLightSeparator           = color.NRGBA{R: 0xe3, G: 0xe3, B: 0xe3, A: 0xff}
	colorLightShadow              = color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x33}
	colorLightSuccess             = color.NRGBA{R: 0x43, G: 0xf4, B: 0x36, A: 0xff}
	colorLightWarning             = color.NRGBA{R: 0xff, G: 0x98, B: 0x00, A: 0xff}
)

// Color looks up the named colour for current theme and variant.
//
// Since: 2.5
func Color(name bellui.ThemeColorName) color.Color {
	return safeColorLookup(name, currentVariant())
}

// ColorForWidget looks up the named colour for the requested widget using the current theme and variant.
// If the widget theme has been overridden that theme will be used.
//
// Since: 2.5
func ColorForWidget(name bellui.ThemeColorName, w core.Widget) color.Color {
	return CurrentForWidget(w).Color(name, currentVariant())
}

// PrimaryColorNames returns a list of the standard primary color options.
//
// Since: 1.4
func PrimaryColorNames() []string {
	return []string{ColorRed, ColorOrange, ColorYellow, ColorGreen, ColorBlue, ColorPurple, ColorBrown, ColorGray}
}

func safeColorLookup(n bellui.ThemeColorName, v bellui.ThemeVariant) color.Color {
	col := Current().Color(n, v)
	if col == nil {
		bellui.LogError("Loaded theme returned nil color", nil)
		return fallbackColor
	}
	return col
}
