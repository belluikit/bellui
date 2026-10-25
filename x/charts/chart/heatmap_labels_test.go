package chart

import (
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"

	"github.com/belluikit/bellui/x/charts/series"
)

// A heatmap draws its row and column labels beside the cells; they were only
// ever shown in the tooltip. The hover still finds the cell under the
// pointer, so the cells it hit-tests are the ones drawn (#343).
func TestHeatmap_DrawsItsLabelsAndHitsTheDrawnCells(t *testing.T) {
	test.NewTempApp(t)
	hs := &series.HeatmapSeries{
		SeriesName: "h", Rows: 2, Cols: 3,
		Data:    []float64{1, 2, 3, 4, 5, 6},
		XLabels: []string{"Mon", "Tue", "Wed"},
		YLabels: []string{"Morning", "Evening"},
	}
	c := New(Option{Series: []series.Series{hs}})
	w := test.NewTempWindow(t, c)
	w.Resize(geo.NewSize(400, 300))
	r := test.TempWidgetRenderer(t, c)
	r.Refresh()

	for _, l := range append(append([]string{}, hs.XLabels...), hs.YLabels...) {
		if findText(r.Objects(), l) == nil {
			t.Errorf("the heatmap does not draw its label %q", l)
		}
	}

	var cells *canvas.Raster
	var walk func([]core.CanvasObject)
	walk = func(objs []core.CanvasObject) {
		for _, o := range objs {
			switch v := o.(type) {
			case *canvas.Raster:
				cells = v
			case interface{ Objects() []core.CanvasObject }:
				walk(v.Objects())
			}
		}
	}
	walk(r.Objects())
	if cells == nil {
		t.Fatal("the heatmap drew no cells")
	}
	// Just left of the drawn boundary between columns 0 and 1, and just above
	// the one between rows 0 and 1: hit-testing the viewport the labels now
	// share would find column 1 here.
	p, s := cells.Position(), cells.Size()
	for _, tc := range []struct {
		pos  geo.Position
		want int
	}{
		{geo.NewPos(p.X+s.Width/3-2, p.Y+s.Height/2-2), 0},
		{geo.NewPos(p.X+s.Width/3+2, p.Y+s.Height/2+2), 4},
	} {
		hit := c.renderSnap.hitTester.FindNearest(tc.pos, 1)
		if hit == nil || hit.DataIndex != tc.want {
			t.Errorf("hovering %v, in the drawn cell with data index %d, hit %+v", tc.pos, tc.want, hit)
		}
	}
	if mon := findText(r.Objects(), "Morning"); mon != nil && mon.Position().X+mon.Size().Width > p.X {
		t.Errorf("the row label ends at %.1f, over the cells from %.1f", mon.Position().X+mon.Size().Width, p.X)
	}
}
