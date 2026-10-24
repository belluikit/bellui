package chart

import (
	"math"
	"sort"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/series"
	"github.com/belluikit/bellui/x/charts/style"
)

// gridLinePositions returns the x of each vertical grid line and the y of each
// horizontal one, sorted.
func gridLinePositions(r *chartRenderer) (xs, ys []float32) {
	gc := style.NewResolver(r.chart.Theme(), bellui.CurrentApp().Settings().ThemeVariant()).GridLineColor()
	for _, o := range r.objects {
		l, ok := o.(*canvas.Line)
		if !ok || l.StrokeWidth != 0.5 || !sameColor(l.StrokeColor, gc) {
			continue
		}
		switch {
		case l.Position1.X == l.Position2.X:
			xs = append(xs, l.Position1.X)
		case l.Position1.Y == l.Position2.Y:
			ys = append(ys, l.Position1.Y)
		}
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
	sort.Slice(ys, func(i, j int) bool { return ys[i] < ys[j] })
	return xs, ys
}

// tickMarkXs returns the x of each X-axis tick mark: the short vertical lines
// in the X axis's LineColor (the axis line itself is horizontal).
func tickMarkXs(r *chartRenderer) []float32 {
	var xs []float32
	for _, o := range r.objects {
		if l, ok := o.(*canvas.Line); ok && sameColor(l.StrokeColor, xLine) && l.Position1.X == l.Position2.X {
			xs = append(xs, l.Position1.X)
		}
	}
	sort.Slice(xs, func(i, j int) bool { return xs[i] < xs[j] })
	return xs
}

func nearPx(a, b float32) bool { return math.Abs(float64(a-b)) < 0.01 }

func assertAt(t *testing.T, what string, got, want []float32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s at %v, want %d lines at %v", what, got, len(want), want)
	}
	for i := range want {
		if !nearPx(got[i], want[i]) {
			t.Fatalf("%s at %v, want %v", what, got, want)
		}
	}
}

// A category axis puts its ticks at the band centres, where the bars sit. Its
// grid lines go between the bands instead (#757): n-1 lines for n categories,
// none on the plot edges, so each line separates two bars rather than running
// through one. The tick marks stay at the centres.
func TestSplitLineShow_CategoryXAxisBetweenBands(t *testing.T) {
	for _, inverse := range []bool{false, true} {
		_, r := renderOpt(t, Option{
			XAxis: []axis.Config{{Type: axis.TypeCategory, Data: []string{"a", "b", "c", "d"}, SplitLine: axis.SplitLineShow, Inverse: inverse, LineColor: xLine}},
			YAxis: []axis.Config{{Type: axis.TypeValue, SplitLine: axis.SplitLineHide}},
			Series: []series.Series{
				&series.BarSeries{SeriesName: "s", Data: []float64{3, 5, 4, 2}},
			},
		})
		p := r.cachedPlotArea
		band := p.W / 4
		xs, ys := gridLinePositions(r)
		if len(ys) != 0 {
			t.Errorf("inverse=%v: horizontal grid lines at %v, want none (Y SplitLineHide)", inverse, ys)
		}
		assertAt(t, "category X grid lines", xs, []float32{p.X + band, p.X + 2*band, p.X + 3*band})
		assertAt(t, "category X tick marks", tickMarkXs(r),
			[]float32{p.X + band/2, p.X + 1.5*band, p.X + 2.5*band, p.X + 3.5*band})
	}
}

// A value X axis keeps its grid lines at its ticks.
func TestSplitLineShow_ValueXAxisAtTicks(t *testing.T) {
	_, r := renderOpt(t, Option{
		XAxis:  []axis.Config{{Type: axis.TypeValue, SplitCount: 5, SplitLine: axis.SplitLineShow, LineColor: xLine}},
		YAxis:  []axis.Config{{Type: axis.TypeValue, SplitLine: axis.SplitLineHide}},
		Series: []series.Series{&series.ScatterSeries{SeriesName: "s", Data: []float64{1, 2, 3, 4, 5, 1}}},
	})
	xs, _ := gridLinePositions(r)
	ticks := tickMarkXs(r)
	if len(ticks) < 3 {
		t.Fatalf("value X axis drew %d tick marks, want several", len(ticks))
	}
	assertAt(t, "value X grid lines", xs, ticks)
}

// A category Y axis is treated the same way: its horizontal grid lines sit
// between its bands, whether SplitLineAuto (the first Y axis) or SplitLineShow
// turned them on.
func TestSplitLine_CategoryYAxisBetweenBands(t *testing.T) {
	for _, sl := range []axis.SplitLine{axis.SplitLineAuto, axis.SplitLineShow} {
		_, r := renderOpt(t, Option{
			XAxis:  []axis.Config{{Type: axis.TypeValue}},
			YAxis:  []axis.Config{{Type: axis.TypeCategory, Data: []string{"a", "b", "c"}, SplitLine: sl}},
			Series: []series.Series{&series.ScatterSeries{SeriesName: "s", Data: []float64{1, 0, 2, 1, 3, 2}}},
		})
		p := r.cachedPlotArea
		band := p.H / 3
		xs, ys := gridLinePositions(r)
		if len(xs) != 0 {
			t.Errorf("SplitLine %d: vertical grid lines at %v, want none", sl, xs)
		}
		assertAt(t, "category Y grid lines", ys, []float32{p.Y + band, p.Y + 2*band})
	}
}

// A zoomed category axis (DataZoom) still draws its lines between the visible
// bands, strictly inside the plot.
func TestSplitTicks_ZoomedCategoryAxis(t *testing.T) {
	s := axis.NewCategoryScale([]string{"a", "b", "c", "d", "e"})
	s.SetRange(0.5, 3.5)
	cfg := axis.Config{Type: axis.TypeCategory}
	for _, tk := range splitTicks(cfg, s) {
		if tk.Norm <= 0 || tk.Norm >= 1 {
			t.Errorf("grid line at norm %v, want strictly inside the plot", tk.Norm)
		}
	}
	if got := len(splitTicks(cfg, s)); got != len(s.Ticks())-1 {
		t.Errorf("%d grid lines for %d visible ticks, want one between each pair", got, len(s.Ticks()))
	}
}
