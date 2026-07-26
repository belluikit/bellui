package theme

import (
	"image/color"
	"log"
	"sync"

	"github.com/belluikit/bellui"
)

// knownColors is the set of all valid color names.
var knownColors = map[bellui.ThemeColorName]bool{
	ColorNameBackground: true, ColorNameButton: true,
	ColorNameCardBackground: true, ColorNameDisabled: true, ColorNameDisabledButton: true,
	ColorNameElevatedBackground: true, ColorNameError: true,
	ColorNameFocus: true, ColorNameForeground: true,
	ColorNameForegroundOnError: true, ColorNameForegroundOnPrimary: true,
	ColorNameForegroundOnSuccess: true, ColorNameForegroundOnWarning: true,
	ColorNameHeaderBackground: true, ColorNameHover: true, ColorNameHyperlink: true,
	ColorNameInputBackground: true, ColorNameInputBorder: true,
	ColorNameMenuBackground: true, ColorNameOverlayBackground: true,
	ColorNamePlaceHolder: true, ColorNamePressed: true, ColorNamePrimary: true,
	ColorNameScrollBar: true, ColorNameScrollBarBackground: true,
	ColorNameSelection: true, ColorNameSeparator: true, ColorNameShadow: true,
	ColorNameSuccess: true, ColorNameWarning: true,
	// MD3 colors.
	ColorNamePrimaryContainer: true, ColorNameOnPrimaryContainer: true,
	ColorNameSecondary: true, ColorNameOnSecondary: true,
	ColorNameSecondaryContainer: true, ColorNameOnSecondaryContainer: true,
	ColorNameTertiary: true, ColorNameOnTertiary: true,
	ColorNameTertiaryContainer: true, ColorNameOnTertiaryContainer: true,
	ColorNameErrorContainer: true, ColorNameOnErrorContainer: true,
	ColorNameSurface: true, ColorNameSurfaceDim: true, ColorNameSurfaceBright: true,
	ColorNameSurfaceContainerLowest: true, ColorNameSurfaceContainerLow: true,
	ColorNameSurfaceContainerMedium: true, ColorNameSurfaceContainerHigh: true,
	ColorNameSurfaceContainerHighest: true,
	ColorNameOnSurface:               true, ColorNameOnSurfaceVariant: true,
	ColorNameOutline: true, ColorNameOutlineVariant: true,
	ColorNameInverseSurface: true, ColorNameInverseOnSurface: true,
	ColorNameInversePrimary: true, ColorNameScrim: true,
}

// knownSizes is the set of all valid size names.
var knownSizes = map[bellui.ThemeSizeName]bool{
	SizeNameCaptionText: true, SizeNameInlineIcon: true, SizeNameInnerPadding: true,
	SizeNameLineSpacing: true, SizeNamePadding: true, SizeNameScrollBar: true,
	SizeNameScrollBarSmall: true, SizeNameSeparatorThickness: true, SizeNameText: true,
	SizeNameHeadingText: true, SizeNameSubHeadingText: true, SizeNameInputBorder: true,
	SizeNameInputRadius: true, SizeNameSelectionRadius: true, SizeNameScrollBarRadius: true,
	SizeNameWindowButtonHeight: true, SizeNameWindowButtonRadius: true,
	SizeNameWindowButtonIcon: true, SizeNameWindowTitleBarHeight: true, SizeNameTabContentMargin: true,
	SizeNameShadowDepth: true, SizeNameButtonRadius: true, SizeNameCardRadius: true,
	// MD3 sizes.
	SizeNameDisplayLarge: true, SizeNameDisplayMedium: true, SizeNameDisplaySmall: true,
	SizeNameHeadlineLarge: true, SizeNameHeadlineMedium: true, SizeNameHeadlineSmall: true,
	SizeNameTitleLarge: true, SizeNameTitleMedium: true, SizeNameTitleSmall: true,
	SizeNameBodyLarge: true, SizeNameBodyMedium: true, SizeNameBodySmall: true,
	SizeNameLabelLarge: true, SizeNameLabelMedium: true, SizeNameLabelSmall: true,
	SizeNameIconButtonSize: true, SizeNameIconSize: true, SizeNameFABSize: true,
	SizeNameChipHeight: true, SizeNameButtonMinHeight: true,
	SizeNameNavBarHeight: true, SizeNameNavRailWidth: true,
	SizeNameTopBarHeight: true, SizeNameBottomBarHeight: true,
	SizeNameSwitchTrackWidth: true,
	SizeNameShapeNone:        true, SizeNameShapeExtraSmall: true, SizeNameShapeSmall: true,
	SizeNameShapeMedium: true, SizeNameShapeLarge: true, SizeNameShapeExtraLarge: true,
	SizeNameShapeFull: true,
}

var warnedKeys sync.Map

// warnUnknownColor logs a warning the first time an unknown color name is used.
func warnUnknownColor(name bellui.ThemeColorName) {
	if knownColors[name] {
		return
	}
	if _, warned := warnedKeys.LoadOrStore("color:"+string(name), true); !warned {
		log.Printf("bellui theme: unknown color name %q — check for typos. Got fallback color.Transparent.", name)
	}
}

// warnUnknownSize logs a warning the first time an unknown size name is used.
func warnUnknownSize(name bellui.ThemeSizeName) {
	if knownSizes[name] {
		return
	}
	if _, warned := warnedKeys.LoadOrStore("size:"+string(name), true); !warned {
		log.Printf("bellui theme: unknown size name %q — check for typos. Got fallback 0.", name)
	}
}

// ValidateTheme checks a theme implementation against all known keys and reports
// any that return fallback values (transparent colors, zero sizes, nil icons).
// Returns a list of issues found. Useful in tests and development.
func ValidateTheme(t bellui.Theme) []string {
	var issues []string
	transparent := color.Transparent

	for name := range knownColors {
		for _, v := range []bellui.ThemeVariant{VariantDark, VariantLight} {
			c := t.Color(name, v)
			if c == nil || c == transparent {
				variant := "dark"
				if v == VariantLight {
					variant = "light"
				}
				issues = append(issues, "missing color: "+string(name)+" ("+variant+")")
			}
		}
	}

	for name := range knownSizes {
		s := t.Size(name)
		if s == 0 {
			issues = append(issues, "zero size: "+string(name))
		}
	}

	return issues
}
