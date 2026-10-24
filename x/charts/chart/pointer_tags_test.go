package chart

import (
	"testing"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/component"
	"github.com/belluikit/bellui/x/charts/series"
)

var pointerTags = &component.TooltipConfig{
	Trigger:       component.TooltipTriggerItem,
	Crosshair:     component.CrosshairBoth,
	ShowAxisTags:  true,
	CrosshairOnly: true,
}

// hoverPoint hovers data point i of the series at seriesIdx and returns the
// objects both render paths draw: a full rebuild, and the partial refresh a
// mouse move takes.
func hoverPoint(t *testing.T, c *Chart, r *chartRenderer, seriesIdx, yAxis, i int, v float64) (full, partial []core.CanvasObject) {
	t.Helper()
	pos := c.renderSnap.coordSystems[yAxis].DataToPixel(float64(i), v)
	if !c.resolveHover(pos) || len(c.tooltipState.Hits) == 0 {
		t.Fatalf("hovering %v found no point", pos)
	}
	if got := c.tooltipState.Hits[0].SeriesIndex; got != seriesIdx {
		t.Fatalf("hovering %v hit series %d, want %d", pos, got, seriesIdx)
	}
	r.fullRebuild(geo.NewSize(400, 300))
	full = r.objects
	r.refreshInteractive()
	return full, r.objects
}

// With the X axis on top and the Y axis on the right, the pointer tags sit
// next to the axes as drawn, not below and to the left (#684, item from #716).
func TestPointerTags_FollowAxisPosition(t *testing.T) {
	test.NewTempApp(t)
	x := categoryX()
	x.Position = axis.PositionTop
	y := valueY()
	y.Position = axis.PositionRight
	c := New(Option{
		XAxis:   []axis.Config{x},
		YAxis:   []axis.Config{y},
		Tooltip: pointerTags,
		Series:  []series.Series{&series.LineSeries{SeriesName: "s", Data: []float64{0, 50, 100}}},
	})
	c.Resize(geo.NewSize(400, 300))
	r := c.CreateRenderer().(*chartRenderer)
	r.fullRebuild(geo.NewSize(400, 300))

	full, partial := hoverPoint(t, c, r, 0, 0, 1, 50)
	plot := c.renderSnap.plotRect
	for name, objs := range map[string][]core.CanvasObject{"full rebuild": full, "partial refresh": partial} {
		xTag, yTag := findText(objs, "1.0"), findText(objs, "50.00")
		if xTag == nil || yTag == nil {
			t.Fatalf("%s: X tag %v, Y tag %v, want both drawn", name, xTag, yTag)
		}
		if b := xTag.Position().Y + xTag.Size().Height; b > plot.Y {
			t.Errorf("%s: X tag ends at y=%v, want it above the plot (top %v), beside the top X axis", name, b, plot.Y)
		}
		if l := yTag.Position().X; l < plot.X+plot.W {
			t.Errorf("%s: Y tag at x=%v, want it right of the plot (right %v), beside the right Y axis", name, l, plot.X+plot.W)
		}
	}
}

// A series on the second Y axis — on the right by default — gets its Y tag
// beside that axis, showing its own value; a series on the first axis keeps
// its tag on the left.
func TestPointerTags_FollowSeriesYAxis(t *testing.T) {
	test.NewTempApp(t)
	lo, hi := 0.0, 100.0
	lo2, hi2 := 0.0, 10.0
	c := New(Option{
		XAxis: []axis.Config{categoryX()},
		YAxis: []axis.Config{
			{Type: axis.TypeValue, Min: &lo, Max: &hi},
			{Type: axis.TypeValue, Min: &lo2, Max: &hi2},
		},
		Tooltip: pointerTags,
		Series: []series.Series{
			&series.LineSeries{SeriesName: "a", Data: []float64{0, 50, 100}},
			&series.LineSeries{SeriesName: "b", Data: []float64{1, 2, 10}, YAxisIndex: 1},
		},
	})
	c.Resize(geo.NewSize(400, 300))
	r := c.CreateRenderer().(*chartRenderer)
	r.fullRebuild(geo.NewSize(400, 300))
	plot := c.renderSnap.plotRect

	full, partial := hoverPoint(t, c, r, 1, 1, 1, 2)
	for name, objs := range map[string][]core.CanvasObject{"full rebuild": full, "partial refresh": partial} {
		yTag := findText(objs, "2.00")
		if yTag == nil {
			t.Fatalf("%s: no Y tag reading series b's value 2.00", name)
		}
		if l := yTag.Position().X; l < plot.X+plot.W {
			t.Errorf("%s: series on Y axis 1: Y tag at x=%v, want it right of the plot (right %v), beside axis 1", name, l, plot.X+plot.W)
		}
	}

	full, partial = hoverPoint(t, c, r, 0, 0, 1, 50)
	for name, objs := range map[string][]core.CanvasObject{"full rebuild": full, "partial refresh": partial} {
		yTag := findText(objs, "50.00")
		if yTag == nil {
			t.Fatalf("%s: no Y tag reading series a's value 50.00", name)
		}
		if e := yTag.Position().X + yTag.Size().Width; e > plot.X {
			t.Errorf("%s: series on Y axis 0: Y tag ends at x=%v, want it left of the plot (left %v)", name, e, plot.X)
		}
	}
}
