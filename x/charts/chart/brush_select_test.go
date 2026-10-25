package chart

import (
	"math"
	"testing"

	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/interact"
	"github.com/belluikit/bellui/x/charts/series"
)

// A brush over bars reports the data range it covers and the bars it
// overlaps. The range was never filled in and bars were never selected (#343).
func TestChart_BrushSelectsBarsAndReportsItsRange(t *testing.T) {
	test.NewTempApp(t)
	c := New(Option{
		XAxis:  []axis.Config{{Type: axis.TypeCategory, Data: []string{"a", "b", "c", "d"}}},
		YAxis:  []axis.Config{axis.DefaultConfig(axis.TypeValue)},
		Series: []series.Series{&series.BarSeries{SeriesName: "s", Data: []float64{10, 20, 30, 40}}},
	})
	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(400, 300))
	test.TempWidgetRenderer(t, c).Refresh()
	cs := c.renderSnap.coordSystems[0]

	// Drag from below-right of bar 2 to above-left of bar 1: the brush covers
	// bars 1 and 2 only, and is dragged backwards on both axes.
	from := cs.DataToPixel(2, 5)
	to := cs.DataToPixel(1, 35)
	c.brushState = interact.BrushState{Active: true, StartPos: from, CurrentPos: to}
	res := c.resolveDragEnd()
	if !res.active {
		t.Fatal("ending the drag produced no brush event")
	}
	evt := res.evt

	var got []int
	for _, p := range evt.Selected {
		got = append(got, p.DataIndex)
	}
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("selected bars %v, want [1 2]", got)
	}

	near := func(a, b float64) bool { return math.Abs(a-b) < 0.01 }
	if !near(evt.DataMinX, 1) || !near(evt.DataMaxX, 2) {
		t.Errorf("X range [%.2f, %.2f], want [1, 2]", evt.DataMinX, evt.DataMaxX)
	}
	if !near(evt.DataMinY, 5) || !near(evt.DataMaxY, 35) {
		t.Errorf("Y range [%.2f, %.2f], want [5, 35]", evt.DataMinY, evt.DataMaxY)
	}
}
