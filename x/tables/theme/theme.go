package theme

import (
	"image/color"

	"github.com/belluikit/bellui"
	fyneTheme "github.com/belluikit/bellui/theme"
)

// GridTheme provides grid-specific theme values, falling back to the app theme.
type GridTheme struct {
	base    bellui.Theme
	density DensityMode
}

// NewGridTheme wraps an existing theme with grid-specific defaults.
func NewGridTheme(base bellui.Theme, density DensityMode) *GridTheme {
	return &GridTheme{base: base, density: density}
}

// Color returns the color for the given name, with grid-specific overrides.
func (t *GridTheme) Color(name bellui.ThemeColorName, variant bellui.ThemeVariant) color.Color {
	switch name {
	case ColorNameHeaderBackground:
		return t.base.Color(fyneTheme.ColorNameHeaderBackground, variant)
	case ColorNameRowSelected:
		return t.base.Color(fyneTheme.ColorNameSelection, variant)
	case ColorNameRowHover:
		return t.base.Color(fyneTheme.ColorNameHover, variant)
	case ColorNameFocusRing:
		return t.base.Color(fyneTheme.ColorNameFocus, variant)
	case ColorNameCellBorder:
		return t.base.Color(fyneTheme.ColorNameSeparator, variant)
	case ColorNameSortIndicator:
		return t.base.Color(fyneTheme.ColorNamePrimary, variant)
	}
	return t.base.Color(name, variant)
}

// Font delegates to the base theme.
func (t *GridTheme) Font(style bellui.TextStyle) bellui.Resource {
	return t.base.Font(style)
}

// Icon delegates to the base theme.
func (t *GridTheme) Icon(name bellui.ThemeIconName) bellui.Resource {
	return t.base.Icon(name)
}

// Size returns the size for the given name, with grid-specific overrides.
func (t *GridTheme) Size(name bellui.ThemeSizeName) float32 {
	v := DefaultSize(name, t.density)
	if v > 0 {
		return v
	}
	return t.base.Size(name)
}
