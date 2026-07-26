package test

import (
	"image/color"

	"github.com/belluikit/bellui"
)

func DarkTheme(fallback bellui.Theme) bellui.Theme {
	return &forcedVariant{Theme: fallback, variant: 0} // avoid import loops
}

func LightTheme(fallback bellui.Theme) bellui.Theme {
	return &forcedVariant{Theme: fallback, variant: 1} // avoid import loops
}

type forcedVariant struct {
	bellui.Theme

	variant bellui.ThemeVariant
}

func (f *forcedVariant) Color(name bellui.ThemeColorName, _ bellui.ThemeVariant) color.Color {
	return f.Theme.Color(name, f.variant)
}
