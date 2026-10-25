package chart

import (
	"fmt"
	"image/color"
	"math"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

type barCollector struct{ bars []series.HitBar }

func (c *barCollector) AddHitPoints(int, string, series.Type, []series.HitPoint) {}
func (c *barCollector) AddHitBars(_ int, _ string, _ series.Type, bars []series.HitBar) {
	c.bars = append(c.bars, bars...)
}

// Bars, candles, boxes and pictorial bars widen as a zoom shows fewer points,
// drawn and hit areas alike; they kept their unzoomed width (#343). Unzoomed they are exactly as
// wide as before, the plot width over the point count, on category and value
// X axes both.
func TestCategoryShapesWidenWithZoom(t *testing.T) {
	const n = 20
	data := make([]float64, n)
	candles := make([]series.CandlestickData, n)
	boxes := make([]series.BoxplotData, n)
	cats := make([]string, n)
	for i := range data {
		data[i] = 50
		candles[i] = series.CandlestickData{Open: 40, Close: 60, Low: 30, High: 70}
		boxes[i] = series.BoxplotData{Min: 30, Q1: 40, Median: 50, Q3: 60, Max: 70}
		cats[i] = fmt.Sprint(i)
	}
	for _, xt := range []axis.Type{axis.TypeCategory, axis.TypeValue} {
		for _, tc := range []struct {
			s    series.Series
			frac float32 // the share of a point's room the shape takes
		}{
			{&series.BarSeries{SeriesName: "b", Data: data}, series.DefaultBarWidth},
			{&series.CandlestickSeries{SeriesName: "c", Data: candles}, series.DefaultBarWidth},
			{&series.BoxplotSeries{SeriesName: "x", Data: boxes}, series.DefaultBoxWidth},
			{&series.PictorialSeries{SeriesName: "p", Data: data}, 0.6},
		} {
			s := tc.s
			t.Run(fmt.Sprintf("%T on X axis type %d", s, xt), func(t *testing.T) {
				test.NewTempApp(t)
				x := axis.DefaultConfig(xt)
				if xt == axis.TypeCategory {
					x.Data = cats
				}
				c := New(Option{
					XAxis: []axis.Config{x}, YAxis: []axis.Config{axis.DefaultConfig(axis.TypeValue)},
					Series:   []series.Series{s},
					DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.01, MaxSpan: 1, ZoomSensitivity: 0.05},
				})
				w := test.NewTempWindow(t, c)
				w.Resize(geo.NewSize(800, 300))
				r := test.TempWidgetRenderer(t, c)
				r.Refresh()

				width := func() (drawn, hit float32) {
					cs := c.renderSnap.coordSystems[0]
					ctx := series.RenderContext{Series: s, Coord: cs, Viewport: cs.Bounds(), Color: color.White}
					for _, o := range series.NewRenderer(s.SeriesType()).Render(ctx) {
						if rect, ok := o.(*canvas.Rectangle); ok && rect.Size().Width > drawn {
							drawn = rect.Size().Width
						}
					}
					reg, ok := s.(series.HitRegistrar)
					if !ok {
						return drawn, drawn // registers no hit areas
					}
					col := &barCollector{}
					reg.RegisterHitAreas(col, 0, cs, cs.Bounds())
					return drawn, col.bars[0].Rect[2]
				}

				fullDrawn, fullHit := width()
				before := c.renderSnap.coordSystems[0].Bounds().W / n * tc.frac
				if math.Abs(float64(fullDrawn-before)) > 0.01 || math.Abs(float64(fullHit-before)) > 0.01 {
					t.Errorf("unzoomed: drawn %.2f, hit area %.2f wide, want %.2f as before", fullDrawn, fullHit, before)
				}

				c.SetZoomWindow(interact.DataWindow{Start: 0.25, End: 0.75})
				r.Refresh()
				zoomDrawn, zoomHit := width()
				// A value axis rounds the zoomed range out to nice numbers, so
				// more than half the points can stay on screen.
				if zoomDrawn < 1.5*fullDrawn {
					t.Errorf("zoomed to half the data, the width grew from %.1f only to %.1f", fullDrawn, zoomDrawn)
				}
				cs := c.renderSnap.coordSystems[0]
				if step := cs.DataToPixel(1, 0).X - cs.DataToPixel(0, 0).X; zoomDrawn >= step {
					t.Errorf("zoomed: %.1f wide with %.1f between points, so neighbours overlap", zoomDrawn, step)
				}
				if math.Abs(float64(zoomHit-zoomDrawn)) > 0.01 {
					t.Errorf("zoomed: hit area %.2f wide, drawn %.2f", zoomHit, zoomDrawn)
				}
			})
		}
	}
}
