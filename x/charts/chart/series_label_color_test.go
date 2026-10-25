package chart

import (
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/belluikit/bellui/x/charts/series"
)

// The chart hands series renderers the theme's foreground as
// RenderContext.TextColor, so a gauge's value reads on a light theme (#343).
func TestChart_SeriesLabelsTakeTheThemeForeground(t *testing.T) {
	for _, v := range []bellui.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		a := test.NewTempApp(t)
		a.Settings().SetTheme(theme.DefaultTheme())
		a.Settings().(interface{ SetThemeVariant(bellui.ThemeVariant) }).SetThemeVariant(v)
		c := New(Option{Series: []series.Series{&series.GaugeSeries{SeriesName: "g", Value: 62, ShowValue: true}}})
		w := test.NewTempWindow(t, c)
		w.Resize(geo.NewSize(300, 220))
		test.TempWidgetRenderer(t, c).Refresh()

		value := findText(test.TempWidgetRenderer(t, c).Objects(), "62.0")
		if value == nil {
			t.Fatalf("variant %d: the gauge drew no value label", v)
		}
		if want := theme.DefaultTheme().Color(theme.ColorNameForeground, v); value.Color != want {
			t.Errorf("variant %d: the gauge value is %v, want the theme's foreground %v", v, value.Color, want)
		}
	}
}
