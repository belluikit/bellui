package editor

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"
)

// hostTheme is what belluikit/studio#27 wants to write: an app theme that
// answers some editor.* names and stays silent on the rest.
type hostTheme struct {
	base   bellui.Theme
	colors map[bellui.ThemeColorName]color.Color
}

func (t *hostTheme) Color(n bellui.ThemeColorName, v bellui.ThemeVariant) color.Color {
	if c, ok := t.colors[n]; ok {
		return c
	}
	return t.base.Color(n, v)
}

func (t *hostTheme) Font(s bellui.TextStyle) bellui.Resource { return t.base.Font(s) }
func (t *hostTheme) Icon(n bellui.ThemeIconName) bellui.Resource {
	return t.base.Icon(n)
}
func (t *hostTheme) Size(n bellui.ThemeSizeName) float32 { return t.base.Size(n) }

// TestThemeColorAppThemeWins is #150's acceptance: a host theme defining an
// editor.* name restyles the editor, instead of the built-in palette
// pre-empting it.
func TestThemeColorAppThemeWins(t *testing.T) {
	a := test.NewTempApp(t)

	want := color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xFF}
	a.Settings().SetTheme(&hostTheme{
		base:   theme.DefaultTheme(),
		colors: map[bellui.ThemeColorName]color.Color{ColorNameSyntaxKeyword: want},
	})

	if got := themeColor(ColorNameSyntaxKeyword); got != want {
		t.Errorf("themeColor(%s) = %v, want the app theme's %v", ColorNameSyntaxKeyword, got, want)
	}
}

// TestThemeColorFallsBackWhenUnanswered: names the host theme does not define
// resolve to the built-in defaults — DefaultTheme's palette answers unknown
// names with color.Transparent, the established "not answered" sentinel.
func TestThemeColorFallsBackWhenUnanswered(t *testing.T) {
	a := test.NewTempApp(t)
	a.Settings().SetTheme(&hostTheme{
		base:   theme.DefaultTheme(),
		colors: map[bellui.ThemeColorName]color.Color{ColorNameSyntaxKeyword: color.Black},
	})

	def := defaultColors()[ColorNameSyntaxString]
	if got := themeColor(ColorNameSyntaxString); got != def {
		t.Errorf("themeColor(%s) = %v, want built-in default %v", ColorNameSyntaxString, got, def)
	}
}

// TestThemeColorTransparentAnswerFallsBack: an explicit Transparent from the
// theme reads as "unanswered" (theme.ValidateTheme's convention), not as a
// deliberate transparent syntax color.
func TestThemeColorTransparentAnswerFallsBack(t *testing.T) {
	a := test.NewTempApp(t)
	a.Settings().SetTheme(&hostTheme{
		base: theme.DefaultTheme(),
		colors: map[bellui.ThemeColorName]color.Color{
			ColorNameSyntaxComment: color.Transparent,
		},
	})

	def := defaultColors()[ColorNameSyntaxComment]
	if got := themeColor(ColorNameSyntaxComment); got != def {
		t.Errorf("themeColor(%s) = %v, want built-in default %v", ColorNameSyntaxComment, got, def)
	}
}

// TestThemeColorVariantReachesBothSides: the variant flows to the app theme's
// Color call, and the built-in fallback flips palettes with it.
func TestThemeColorVariantReachesBothSides(t *testing.T) {
	a := test.NewTempApp(t)

	perVariant := map[bellui.ThemeVariant]color.Color{
		theme.VariantDark:  color.NRGBA{R: 0x11, A: 0xFF},
		theme.VariantLight: color.NRGBA{R: 0x22, A: 0xFF},
	}
	a.Settings().SetTheme(&variantEcho{
		base: theme.DefaultTheme(), perVariant: perVariant,
	})

	set := a.Settings()

	set.SetThemeVariant(theme.VariantDark)
	if got := themeColor(ColorNameSyntaxKeyword); got != perVariant[theme.VariantDark] {
		t.Errorf("dark variant: themeColor = %v, want %v", got, perVariant[theme.VariantDark])
	}
	if got := themeColor(ColorNameSyntaxString); got != defaultDarkColors[ColorNameSyntaxString] {
		t.Errorf("dark variant fallback = %v, want the dark default", got)
	}

	set.SetThemeVariant(theme.VariantLight)
	if got := themeColor(ColorNameSyntaxKeyword); got != perVariant[theme.VariantLight] {
		t.Errorf("light variant: themeColor = %v, want %v", got, perVariant[theme.VariantLight])
	}
	if got := themeColor(ColorNameSyntaxString); got != defaultLightColors[ColorNameSyntaxString] {
		t.Errorf("light variant fallback = %v, want the light default", got)
	}
}

// variantEcho answers ColorNameSyntaxKeyword with a per-variant color and
// stays silent otherwise, proving the variant parameter arrives.
type variantEcho struct {
	base       bellui.Theme
	perVariant map[bellui.ThemeVariant]color.Color
}

func (t *variantEcho) Color(n bellui.ThemeColorName, v bellui.ThemeVariant) color.Color {
	if n == ColorNameSyntaxKeyword {
		if c, ok := t.perVariant[v]; ok {
			return c
		}
	}
	return t.base.Color(n, v)
}

func (t *variantEcho) Font(s bellui.TextStyle) bellui.Resource { return t.base.Font(s) }
func (t *variantEcho) Icon(n bellui.ThemeIconName) bellui.Resource {
	return t.base.Icon(n)
}
func (t *variantEcho) Size(n bellui.ThemeSizeName) float32 { return t.base.Size(n) }
