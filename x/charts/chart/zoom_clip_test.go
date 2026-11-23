package chart

import (
	"fmt"
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

// xExtent is how far left and right o draws.
func xExtent(o core.CanvasObject) (float32, float32) {
	if l, ok := o.(*canvas.Line); ok {
		return min(l.Position1.X, l.Position2.X), max(l.Position1.X, l.Position2.X)
	}
	return o.Position().X, o.Position().X + o.Size().Width
}

// clipSeries are the series #1396 clips, on the gallery's Trading price and
// volume data: 13 categories.
func clipSeries() []series.Series {
	const n = 13
	data := make([]float64, n)
	var pairs []float64
	candles := make([]series.CandlestickData, n)
	boxes := make([]series.BoxplotData, n)
	for i := range data {
		data[i] = float64(1000 + 100*i)
		pairs = append(pairs, float64(i), data[i])
		o := 150 + float64(i)
		candles[i] = series.CandlestickData{Open: o, Close: o + 3, Low: o - 2, High: o + 5}
		boxes[i] = series.BoxplotData{Min: 30, Q1: 40, Median: 50, Q3: 60, Max: 70}
	}
	return []series.Series{
		&series.BarSeries{SeriesName: "bar", Data: data},
		&series.CandlestickSeries{SeriesName: "candlestick", Data: candles},
		&series.BoxplotSeries{SeriesName: "boxplot", Data: boxes},
		&series.PictorialSeries{SeriesName: "pictorial", Data: data},
		&series.ScatterSeries{SeriesName: "scatter", Data: pairs},
		&series.EffectScatterSeries{SeriesName: "effect scatter", Data: pairs},
	}
}

// renderZoomed renders s into a 13-category chart zoomed to start..end and
// returns what the series drew, as the chart draws it, with the plot.
func renderZoomed(t *testing.T, s series.Series, start, end float64) ([]core.CanvasObject, coord.Rect, coord.System) {
	t.Helper()
	cats := make([]string, 13)
	for i := range cats {
		cats[i] = fmt.Sprint(i)
	}
	c := New(Option{
		XAxis: []axis.Config{{Type: axis.TypeCategory, Data: cats}}, YAxis: []axis.Config{{Type: axis.TypeValue}},
		Series:   []series.Series{s},
		DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.05, MaxSpan: 1},
	})
	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(800, 400))
	r := test.TempWidgetRenderer(t, c)
	c.SetZoomWindow(interact.DataWindow{Start: start, End: end})
	r.Refresh()
	cs := c.renderSnap.coordSystems[0]
	b := cs.Bounds()
	ctx := series.RenderContext{Series: s, Coord: cs, Viewport: b, Color: color.White}
	return series.NewRenderer(s.SeriesType()).Render(ctx), b, cs
}

// Zoomed in, nothing a bar, candle, box, pictorial or scatter series draws
// reaches past the plot's left or right edge, over a sweep of windows down
// to the zoom limit. They drew the hidden points outside the plot, and a
// candle or bar partly in view whole: 8 of 13 bars and up to 272 px of a
// candle in the gallery's Trading chart (#1396). A rectangle is cut at the
// edge; a symbol may overhang by up to half its size.
func TestZoomedShapesStayInPlot(t *testing.T) {
	test.NewTempApp(t)
	for _, s := range clipSeries() {
		t.Run(s.Name(), func(t *testing.T) {
			drawn := 0
			for start := 0.0; start < 0.95; start += 0.05 {
				for _, span := range []float64{0.06, 0.1, 0.2, 0.35, 0.5} {
					if start+span > 1 {
						continue
					}
					objs, b, _ := renderZoomed(t, s, start, start+span)
					drawn += len(objs)
					for _, o := range objs {
						x0, x1 := xExtent(o)
						over := float32(0.5)
						if _, isRect := o.(*canvas.Rectangle); !isRect {
							if _, isLine := o.(*canvas.Line); !isLine {
								over += o.Size().Width / 2 // a symbol
							}
						}
						if x0 < b.X-over || x1 > b.X+b.W+over {
							t.Fatalf("window %.2f..%.2f: a %T at x %.1f..%.1f, plot x %.1f..%.1f", start, start+span, o, x0, x1, b.X, b.X+b.W)
						}
					}
				}
			}
			if drawn == 0 {
				t.Fatal("nothing was drawn, so nothing was checked")
			}
		})
	}
}

// Unzoomed, the clip changes nothing: every series draws exactly what it
// draws in a viewport too wide to clip anything.
func TestUnzoomedShapesUnclipped(t *testing.T) {
	test.NewTempApp(t)
	for _, s := range clipSeries() {
		t.Run(s.Name(), func(t *testing.T) {
			clipped, b, cs := renderZoomed(t, s, 0, 1)
			wide := coord.Rect{X: -1e6, Y: b.Y, W: 2e6, H: b.H}
			free := series.NewRenderer(s.SeriesType()).Render(series.RenderContext{Series: s, Coord: cs, Viewport: wide, Color: color.White})
			if len(clipped) != len(free) {
				t.Fatalf("clipped drew %d objects, unclipped %d", len(clipped), len(free))
			}
			for i := range free {
				a0, a1 := xExtent(clipped[i])
				b0, b1 := xExtent(free[i])
				if a0 != b0 || a1 != b1 {
					t.Errorf("object %d (%T): x %.1f..%.1f clipped, %.1f..%.1f unclipped", i, free[i], a0, a1, b0, b1)
				}
			}
		})
	}
}
