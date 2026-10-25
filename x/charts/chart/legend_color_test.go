package chart

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/theme"

	"github.com/belluikit/bellui/x/charts/series"
	"github.com/belluikit/bellui/x/charts/style"
)

// A series that sets its own colour shows that colour in the legend. The
// legend showed the palette colour for its position, so a 3D line chart drawn
// blue, pink and green listed lavender, blue and yellow (#343, from #403).
func TestLegend_ShowsTheSeriesOwnColor(t *testing.T) {
	test.NewTempApp(t)
	pink := color.NRGBA{R: 255, G: 105, B: 180, A: 255}
	green := color.NRGBA{G: 200, A: 255}
	c := New(Option{Series: []series.Series{
		&series.LineSeries{SeriesName: "line", Data: []float64{1, 2}, LineStyle: series.LineStyle{Color: pink}},
		&series.Line3DSeries{SeriesName: "line3d", Data: []float64{0, 0, 0, 1, 1, 1}, Color: green},
		&series.BarSeries{SeriesName: "palette", Data: []float64{1, 2}},
	}})
	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(400, 300))
	r := test.TempWidgetRenderer(t, c).(*chartRenderer)

	resolver := style.NewResolver(theme.Current(), theme.VariantDark)
	items := r.buildLegendItems(c.option, resolver)
	if len(items) != 3 {
		t.Fatalf("%d legend items, want 3", len(items))
	}
	for i, want := range []color.Color{pink, green, resolver.SeriesColor(2)} {
		if items[i].Color != want {
			t.Errorf("legend item %q is %v, want %v", items[i].Name, items[i].Color, want)
		}
	}
}

// Every series type with a colour of its own reports it; nil means palette.
func TestColoredSeries_ReportsItsColor(t *testing.T) {
	c := color.NRGBA{R: 1, G: 2, B: 3, A: 255}
	for _, s := range []series.ColoredSeries{
		&series.LineSeries{LineStyle: series.LineStyle{Color: c}},
		&series.BarSeries{Color: c},
		&series.ScatterSeries{Color: c},
		&series.EffectScatterSeries{Color: c},
		&series.BoxplotSeries{Color: c},
		&series.PictorialSeries{Color: c},
		&series.GeoScatterSeries{Color: c},
		&series.GeoLinesSeries{Color: c},
		&series.Scatter3DSeries{Color: c},
		&series.Line3DSeries{Color: c},
	} {
		if got := s.SeriesColor(); got != c {
			t.Errorf("%T.SeriesColor() = %v, want %v", s, got, c)
		}
	}
	if got := (&series.BarSeries{}).SeriesColor(); got != nil {
		t.Errorf("a bar with no colour reports %v, want nil", got)
	}
}
