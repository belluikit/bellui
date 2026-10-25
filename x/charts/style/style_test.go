package style

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/theme"
)

// testTheme wraps the default theme for testing.
type testTheme struct{}

func (t *testTheme) Color(name bellui.ThemeColorName, variant bellui.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, variant)
}

func (t *testTheme) Font(style bellui.TextStyle) bellui.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *testTheme) Icon(name bellui.ThemeIconName) bellui.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *testTheme) Size(name bellui.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

func TestSeriesColor_DefaultPalette(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	c0 := r.SeriesColor(0)
	c1 := r.SeriesColor(1)
	if c0 == nil {
		t.Fatal("SeriesColor(0) returned nil")
	}
	if c0 == c1 {
		t.Error("series 0 and 1 should have different colors")
	}
}

func TestSeriesColor_PaletteCycling(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	pal := []color.Color{color.White, color.Black}
	r.SetPalette(pal)

	if r.SeriesColor(0) != color.White {
		t.Error("index 0 should be white")
	}
	if r.SeriesColor(1) != color.Black {
		t.Error("index 1 should be black")
	}
	// Should cycle
	if r.SeriesColor(2) != color.White {
		t.Error("index 2 should cycle to white")
	}
}

func TestBackgroundColor(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	bg := r.BackgroundColor()
	if bg == nil {
		t.Error("BackgroundColor returned nil")
	}
}

func TestGridLineColor(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	c := r.GridLineColor()
	if c == nil {
		t.Error("GridLineColor returned nil")
	}
}

func TestLabelTextSize(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	size := r.LabelTextSize()
	if size <= 0 {
		t.Errorf("LabelTextSize = %v, expected positive", size)
	}
}

func TestTitleTextSize(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	size := r.TitleTextSize()
	if size <= 0 {
		t.Errorf("TitleTextSize = %v, expected positive", size)
	}
}

func TestTooltipBackgroundColor(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	c := r.TooltipBackgroundColor()
	if c == nil {
		t.Error("TooltipBackgroundColor returned nil")
	}
	nrgba, ok := c.(color.NRGBA)
	if !ok {
		t.Fatal("TooltipBackgroundColor did not return color.NRGBA")
	}
	if nrgba.R != 40 || nrgba.G != 40 || nrgba.B != 40 || nrgba.A != 230 {
		t.Errorf("TooltipBackgroundColor = %v, unexpected value", nrgba)
	}
}

func TestTooltipBorderColor(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	c := r.TooltipBorderColor()
	if c == nil {
		t.Error("TooltipBorderColor returned nil")
	}
	nrgba, ok := c.(color.NRGBA)
	if !ok {
		t.Fatal("TooltipBorderColor did not return color.NRGBA")
	}
	if nrgba.R != 80 || nrgba.G != 80 || nrgba.B != 80 || nrgba.A != 200 {
		t.Errorf("TooltipBorderColor = %v, unexpected value", nrgba)
	}
}

func TestTooltipTextColor(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	c := r.TooltipTextColor()
	if c == nil {
		t.Error("TooltipTextColor returned nil")
	}
	nrgba, ok := c.(color.NRGBA)
	if !ok {
		t.Fatal("TooltipTextColor did not return color.NRGBA")
	}
	if nrgba.R != 230 || nrgba.G != 230 || nrgba.B != 230 || nrgba.A != 255 {
		t.Errorf("TooltipTextColor = %v, unexpected value", nrgba)
	}
}

func TestCrosshairColor(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	c := r.CrosshairColor()
	if c == nil {
		t.Error("CrosshairColor returned nil")
	}
	nrgba, ok := c.(color.NRGBA)
	if !ok {
		t.Fatal("CrosshairColor did not return color.NRGBA")
	}
	if nrgba.R != 150 || nrgba.G != 150 || nrgba.B != 150 || nrgba.A != 100 {
		t.Errorf("CrosshairColor = %v, unexpected value", nrgba)
	}
}

func TestTooltipPadding(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	v := r.TooltipPadding()
	if v != 8 {
		t.Errorf("TooltipPadding = %v, expected 8", v)
	}
}

func TestTooltipLineHeight(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	v := r.TooltipLineHeight()
	if v != 16 {
		t.Errorf("TooltipLineHeight = %v, expected 16", v)
	}
}

func TestTooltipMarkerSize(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	v := r.TooltipMarkerSize()
	if v != 8 {
		t.Errorf("TooltipMarkerSize = %v, expected 8", v)
	}
}

func TestOverlayDefaultLineColor(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	c := r.OverlayDefaultLineColor()
	if c == nil {
		t.Error("OverlayDefaultLineColor returned nil")
	}
	nrgba, ok := c.(color.NRGBA)
	if !ok {
		t.Fatal("OverlayDefaultLineColor did not return color.NRGBA")
	}
	if nrgba.R != 255 || nrgba.G != 100 || nrgba.B != 100 || nrgba.A != 200 {
		t.Errorf("OverlayDefaultLineColor = %v, unexpected value", nrgba)
	}
}

func TestOverlayDefaultBandColor(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	c := r.OverlayDefaultBandColor()
	if c == nil {
		t.Error("OverlayDefaultBandColor returned nil")
	}
	nrgba, ok := c.(color.NRGBA)
	if !ok {
		t.Fatal("OverlayDefaultBandColor did not return color.NRGBA")
	}
	if nrgba.R != 91 || nrgba.G != 143 || nrgba.B != 249 || nrgba.A != 40 {
		t.Errorf("OverlayDefaultBandColor = %v, unexpected value", nrgba)
	}
}

func TestOverlayLabelColor(t *testing.T) {
	for _, v := range []bellui.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		r := NewResolver(&testTheme{}, v)
		if got, want := r.OverlayLabelColor(), theme.DefaultTheme().Color(theme.ColorNameForeground, v); got != want {
			t.Errorf("variant %d: OverlayLabelColor = %v, want the theme's %v", v, got, want)
		}
	}
}

func TestAnnotationBackgroundColor(t *testing.T) {
	for _, v := range []bellui.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		r := NewResolver(&testTheme{}, v)
		if got, want := r.AnnotationBackgroundColor(), theme.DefaultTheme().Color(theme.ColorNameOverlayBackground, v); got != want {
			t.Errorf("variant %d: AnnotationBackgroundColor = %v, want the theme's %v", v, got, want)
		}
	}
}

func TestBreadcrumbBackgroundColor(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	c := r.BreadcrumbBackgroundColor()
	if c == nil {
		t.Error("BreadcrumbBackgroundColor returned nil")
	}
	nrgba, ok := c.(color.NRGBA)
	if !ok {
		t.Fatal("BreadcrumbBackgroundColor did not return color.NRGBA")
	}
	if nrgba.R != 70 || nrgba.G != 70 || nrgba.B != 70 || nrgba.A != 220 {
		t.Errorf("BreadcrumbBackgroundColor = %v, unexpected value", nrgba)
	}
}

func TestBreadcrumbTextColor(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	c := r.BreadcrumbTextColor()
	if c == nil {
		t.Error("BreadcrumbTextColor returned nil")
	}
	nrgba, ok := c.(color.NRGBA)
	if !ok {
		t.Fatal("BreadcrumbTextColor did not return color.NRGBA")
	}
	if nrgba.R != 220 || nrgba.G != 220 || nrgba.B != 220 || nrgba.A != 255 {
		t.Errorf("BreadcrumbTextColor = %v, unexpected value", nrgba)
	}
}

func TestBreadcrumbAccentColor(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	c := r.BreadcrumbAccentColor()
	if c == nil {
		t.Error("BreadcrumbAccentColor returned nil")
	}
	nrgba, ok := c.(color.NRGBA)
	if !ok {
		t.Fatal("BreadcrumbAccentColor did not return color.NRGBA")
	}
	if nrgba.R != 91 || nrgba.G != 143 || nrgba.B != 249 || nrgba.A != 255 {
		t.Errorf("BreadcrumbAccentColor = %v, unexpected value", nrgba)
	}
}

func TestBreadcrumbSeparatorColor(t *testing.T) {
	r := NewResolver(&testTheme{}, 0)
	c := r.BreadcrumbSeparatorColor()
	if c == nil {
		t.Error("BreadcrumbSeparatorColor returned nil")
	}
	nrgba, ok := c.(color.NRGBA)
	if !ok {
		t.Fatal("BreadcrumbSeparatorColor did not return color.NRGBA")
	}
	if nrgba.R != 120 || nrgba.G != 120 || nrgba.B != 120 || nrgba.A != 200 {
		t.Errorf("BreadcrumbSeparatorColor = %v, unexpected value", nrgba)
	}
}

// Legend text follows the theme; a fixed light grey was nearly invisible on a
// light theme (#343).
func TestLegendTextColors(t *testing.T) {
	for _, v := range []bellui.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		r := NewResolver(&testTheme{}, v)
		if got, want := r.LegendTextColor(), theme.DefaultTheme().Color(theme.ColorNameForeground, v); got != want {
			t.Errorf("variant %d: LegendTextColor = %v, want the theme's foreground %v", v, got, want)
		}
		if got, want := r.LegendDimmedTextColor(), theme.DefaultTheme().Color(theme.ColorNameDisabled, v); got != want {
			t.Errorf("variant %d: LegendDimmedTextColor = %v, want the theme's disabled %v", v, got, want)
		}
	}
}
