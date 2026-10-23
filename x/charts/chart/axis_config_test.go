package chart

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/event"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

// On an Inverse X axis the data runs right to left, so dragging the plot to
// the right must move the window towards later data, the opposite of a normal
// axis: the content follows the pointer either way.
func TestAxisConfig_InversePanFollowsPointer(t *testing.T) {
	pan := func(inverse bool) float64 {
		test.NewTempApp(t)
		x := categoryX()
		x.Inverse = inverse
		c := New(Option{
			DataZoom: &interact.ZoomConfig{Enabled: true, MinSpan: 0.01, MaxSpan: 1, ZoomSensitivity: 0.05},
			XAxis:    []axis.Config{x},
			Series:   []series.Series{&series.LineSeries{SeriesName: "s", Data: []float64{1, 2, 3}}},
		})
		c.Resize(geo.NewSize(400, 300))
		c.SetZoomWindow(interact.DataWindow{Start: 0.4, End: 0.6})
		c.Dragged(&event.DragEvent{Dragged: geo.Delta{DX: 40}})
		return c.ZoomWindow().Start
	}
	if got := pan(false); got >= 0.4 {
		t.Errorf("normal axis: dragging right moved the window start to %v, want it before 0.4", got)
	}
	if got := pan(true); got <= 0.4 {
		t.Errorf("inverse axis: dragging right moved the window start to %v, want it after 0.4", got)
	}
}

// The axis.Config fields below were declared and documented but never read by
// the renderer (#339): LineColor, LabelColor, Name, Position and Inverse.

func renderAxisChart(t *testing.T, x, y axis.Config) (*chartRenderer, []core.CanvasObject) {
	t.Helper()
	test.NewTempApp(t)
	c := New(Option{
		XAxis: []axis.Config{x},
		YAxis: []axis.Config{y},
		Series: []series.Series{
			&series.LineSeries{SeriesName: "s", Data: []float64{0, 50, 100}},
		},
	})
	c.Resize(geo.NewSize(400, 300))
	r := c.CreateRenderer().(*chartRenderer)
	r.fullRebuild(geo.NewSize(400, 300))
	return r, r.objects
}

func categoryX() axis.Config {
	return axis.Config{Type: axis.TypeCategory, Data: []string{"Mon", "Tue", "Wed"}}
}

func valueY() axis.Config {
	cfg := axis.DefaultConfig(axis.TypeValue)
	lo, hi := 0.0, 100.0
	cfg.Min, cfg.Max = &lo, &hi
	return cfg
}

func findText(objs []core.CanvasObject, s string) *canvas.Text {
	for _, o := range objs {
		if txt, ok := o.(*canvas.Text); ok && txt.Text == s {
			return txt
		}
	}
	return nil
}

func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == b
	}
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

func TestAxisConfig_LineAndLabelColor(t *testing.T) {
	green := color.NRGBA{G: 200, A: 255}
	blue := color.NRGBA{B: 200, A: 255}
	x := categoryX()
	x.LineColor, x.LabelColor = green, blue
	y := valueY()
	y.LineColor, y.LabelColor = green, blue
	r, objs := renderAxisChart(t, x, y)
	plot := r.chart.renderSnap.plotRect

	for _, label := range []string{"Tue", "100"} {
		txt := findText(objs, label)
		if txt == nil {
			t.Fatalf("no tick label %q drawn", label)
		}
		if !sameColor(txt.Color, blue) {
			t.Errorf("tick label %q colour = %v, want LabelColor %v", label, txt.Color, blue)
		}
	}

	var bottom, left bool
	for _, o := range objs {
		l, ok := o.(*canvas.Line)
		if !ok || !sameColor(l.StrokeColor, green) {
			continue
		}
		if l.Position1.Y == plot.Y+plot.H && l.Position2.Y == plot.Y+plot.H && l.Position2.X-l.Position1.X == plot.W {
			bottom = true
		}
		if l.Position1.X == plot.X && l.Position2.X == plot.X && l.Position2.Y-l.Position1.Y == plot.H {
			left = true
		}
	}
	if !bottom || !left {
		t.Errorf("axis lines in LineColor: bottom=%v left=%v, want both", bottom, left)
	}
}

func TestAxisConfig_Name(t *testing.T) {
	x := categoryX()
	x.Name = "Weekday"
	y := valueY()
	y.Name = "Sales"
	r, objs := renderAxisChart(t, x, y)
	plot := r.chart.renderSnap.plotRect

	xn := findText(objs, "Weekday")
	if xn == nil {
		t.Fatal("X axis name not drawn")
	}
	if xn.Position().Y <= plot.Y+plot.H {
		t.Errorf("X axis name at y=%v, want below the plot (bottom %v)", xn.Position().Y, plot.Y+plot.H)
	}
	yn := findText(objs, "Sales")
	if yn == nil {
		t.Fatal("Y axis name not drawn")
	}
	if yn.Position().Y >= plot.Y {
		t.Errorf("Y axis name at y=%v, want above the plot (top %v)", yn.Position().Y, plot.Y)
	}
}

func TestAxisConfig_Inverse(t *testing.T) {
	y := valueY()
	y.Inverse = true
	r, objs := renderAxisChart(t, categoryX(), y)

	top, bottom := findText(objs, "0"), findText(objs, "100")
	if top == nil || bottom == nil {
		t.Fatal("tick labels 0 and 100 not drawn")
	}
	if top.Position().Y >= bottom.Position().Y {
		t.Errorf("inverse Y axis: label 0 at y=%v, label 100 at y=%v; want 0 above 100",
			top.Position().Y, bottom.Position().Y)
	}

	cs := r.chart.renderSnap.coordSystems[0]
	if lo, hi := cs.DataToPixel(0, 0), cs.DataToPixel(0, 100); lo.Y >= hi.Y {
		t.Errorf("inverse Y axis maps 0 to y=%v and 100 to y=%v; want 0 at the top", lo.Y, hi.Y)
	}

	x := categoryX()
	x.Inverse = true
	r, objs = renderAxisChart(t, x, valueY())
	mon, wed := findText(objs, "Mon"), findText(objs, "Wed")
	if mon == nil || wed == nil {
		t.Fatal("category labels not drawn")
	}
	if mon.Position().X <= wed.Position().X {
		t.Errorf("inverse X axis: Mon at x=%v, Wed at x=%v; want Mon on the right", mon.Position().X, wed.Position().X)
	}
	cs = r.chart.renderSnap.coordSystems[0]
	if first, last := cs.DataToPixel(0, 0), cs.DataToPixel(2, 0); first.X <= last.X {
		t.Errorf("inverse X axis maps category 0 to x=%v and 2 to x=%v; want 0 on the right", first.X, last.X)
	}
}

func TestAxisConfig_Position(t *testing.T) {
	x := categoryX()
	x.Position = axis.PositionTop
	y := valueY()
	y.Position = axis.PositionRight
	r, objs := renderAxisChart(t, x, y)
	plot := r.chart.renderSnap.plotRect

	if tue := findText(objs, "Tue"); tue == nil || tue.Position().Y >= plot.Y {
		t.Errorf("X axis at PositionTop: label Tue = %v, want it above the plot (top %v)", tue, plot.Y)
	}
	if hundred := findText(objs, "100"); hundred == nil || hundred.Position().X < plot.X+plot.W {
		t.Errorf("Y axis at PositionRight: label 100 = %v, want it right of the plot (right %v)", hundred, plot.X+plot.W)
	}
}

// The zero Position is PositionBottom, which means nothing for a Y axis: it
// keeps the default side, left for the first Y axis and right for the others.
func TestAxisConfig_DefaultYPositions(t *testing.T) {
	test.NewTempApp(t)
	lo, hi := 0.0, 100.0
	lo2, hi2 := 0.0, 10.0
	c := New(Option{
		XAxis: []axis.Config{categoryX()},
		YAxis: []axis.Config{
			{Type: axis.TypeValue, Min: &lo, Max: &hi},
			{Type: axis.TypeValue, Min: &lo2, Max: &hi2},
		},
		Series: []series.Series{
			&series.LineSeries{SeriesName: "a", Data: []float64{0, 50, 100}},
			&series.LineSeries{SeriesName: "b", Data: []float64{1, 2, 10}, YAxisIndex: 1},
		},
	})
	c.Resize(geo.NewSize(400, 300))
	r := c.CreateRenderer().(*chartRenderer)
	r.fullRebuild(geo.NewSize(400, 300))
	plot := c.renderSnap.plotRect

	if l := findText(r.objects, "100"); l == nil || l.Position().X >= plot.X {
		t.Errorf("first Y axis label 100 = %v, want it left of the plot (left %v)", l, plot.X)
	}
	if l := findText(r.objects, "10"); l == nil || l.Position().X < plot.X+plot.W {
		t.Errorf("second Y axis label 10 = %v, want it right of the plot (right %v)", l, plot.X+plot.W)
	}
}
