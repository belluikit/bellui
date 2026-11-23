package chart

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/belluikit/bellui/x/charts/series"
)

// drawnBackground is the colour of the rectangle a full rebuild puts first.
func drawnBackground(t *testing.T, c *Chart) color.Color {
	t.Helper()
	objs := test.TempWidgetRenderer(t, c).Objects()
	if len(objs) == 0 {
		t.Fatal("the chart drew nothing")
	}
	bg, ok := objs[0].(*canvas.Rectangle)
	if !ok {
		t.Fatalf("first chart object is %T, want the background *canvas.Rectangle", objs[0])
	}
	return color.NRGBAModel.Convert(bg.FillColor)
}

// A chart follows a theme change: after the variant flips, or the app's theme
// is replaced, it draws in the new colours. It kept the colours of its first
// rebuild, because the theme walk only calls Refresh, and Refresh returned
// early with no dirty flag set (#1381).
func TestChart_FollowsThemeChange(t *testing.T) {
	a := test.NewTempApp(t)
	a.Settings().SetTheme(theme.DefaultTheme())
	a.Settings().SetThemeVariant(theme.VariantDark)

	c := New(Option{Series: []series.Series{
		&series.GaugeSeries{SeriesName: "g", Value: 40, Min: 0, Max: 100},
	}})
	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(400, 300))

	bgFor := func(th bellui.Theme, v bellui.ThemeVariant) color.Color {
		return color.NRGBAModel.Convert(th.Color(theme.ColorNameBackground, v))
	}
	dark, light := bgFor(theme.DefaultTheme(), theme.VariantDark), bgFor(theme.DefaultTheme(), theme.VariantLight)
	if dark == light {
		t.Fatalf("the default theme's dark and light backgrounds are both %v; the test cannot tell them apart", dark)
	}
	if got := drawnBackground(t, c); got != dark {
		t.Fatalf("before the switch the background is %v, want dark %v", got, dark)
	}
	before := test.TempWidgetRenderer(t, c).Objects()[0]
	c.Refresh()
	if after := test.TempWidgetRenderer(t, c).Objects()[0]; after != before {
		t.Error("a Refresh with nothing changed rebuilt the chart")
	}

	a.Settings().SetThemeVariant(theme.VariantLight)
	if got := drawnBackground(t, c); got != light {
		t.Errorf("after switching to light the background is %v, want %v", got, light)
	}

	a.Settings().SetThemeVariant(theme.VariantDark)
	if got := drawnBackground(t, c); got != dark {
		t.Errorf("after switching back to dark the background is %v, want %v", got, dark)
	}

	replaced := test.NewTheme()
	want := bgFor(replaced, theme.VariantDark)
	if want == dark {
		t.Fatalf("the replacement theme's background is the default's %v; the test cannot tell them apart", want)
	}
	a.Settings().SetTheme(replaced)
	if got := drawnBackground(t, c); got != want {
		t.Errorf("after replacing the theme the background is %v, want %v", got, want)
	}
}

// mapTheme cannot be compared with ==: it holds a map.
type mapTheme struct {
	bellui.Theme
	colors map[bellui.ThemeColorName]color.Color
}

func (m mapTheme) Color(n bellui.ThemeColorName, v bellui.ThemeVariant) color.Color {
	if c, ok := m.colors[n]; ok {
		return c
	}
	return m.Theme.Color(n, v)
}

// A theme whose type cannot be compared does not panic the chart, and
// replacing it with another of the same type is still noticed, by its colours.
// A Refresh with nothing changed keeps the drawn objects.
func TestChart_FollowsThemeChange_UncomparableTheme(t *testing.T) {
	a := test.NewTempApp(t)
	red := color.NRGBA{R: 200, A: 255}
	blue := color.NRGBA{B: 200, A: 255}
	a.Settings().SetTheme(mapTheme{theme.DefaultTheme(), map[bellui.ThemeColorName]color.Color{theme.ColorNameBackground: red}})

	c := New(Option{Series: []series.Series{
		&series.GaugeSeries{SeriesName: "g", Value: 40, Min: 0, Max: 100},
	}})
	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(400, 300))
	if got := drawnBackground(t, c); got != red {
		t.Fatalf("background %v, want %v", got, red)
	}

	before := test.TempWidgetRenderer(t, c).Objects()[0]
	c.Refresh()
	if after := test.TempWidgetRenderer(t, c).Objects()[0]; after != before {
		t.Error("a Refresh with nothing changed rebuilt the chart")
	}

	a.Settings().SetTheme(mapTheme{theme.DefaultTheme(), map[bellui.ThemeColorName]color.Color{theme.ColorNameBackground: blue}})
	if got := drawnBackground(t, c); got != blue {
		t.Errorf("after replacing the theme the background is %v, want %v", got, blue)
	}
}
