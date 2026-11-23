package series

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/coord"
)

// A context with no viewport, as a caller drawing a series on its own may
// pass, clips nothing (#1396).
func TestClipXNoViewport(t *testing.T) {
	r := canvas.NewRectangle(color.White)
	r.Move(geo.NewPos(-50, 0))
	r.Resize(geo.NewSize(500, 10))
	got := clipX([]core.CanvasObject{r}, coord.Rect{})
	if len(got) != 1 || r.Position().X != -50 || r.Size().Width != 500 {
		t.Fatalf("clipped with no viewport: %d objects, x %v width %v", len(got), r.Position().X, r.Size().Width)
	}
}

// A sloping line is cut where it crosses each edge, its Y following the
// slope, whichever end it was drawn from.
func TestClipXCutsSlopingLines(t *testing.T) {
	vp := coord.Rect{X: 100, Y: 0, W: 200, H: 100}
	for _, l := range []*canvas.Line{
		{Position1: geo.NewPos(0, 0), Position2: geo.NewPos(400, 40)},
		{Position1: geo.NewPos(400, 40), Position2: geo.NewPos(0, 0)},
	} {
		got := clipX([]core.CanvasObject{l}, vp)
		if len(got) != 1 {
			t.Fatalf("line dropped")
		}
		lo, hi := l.Position1, l.Position2
		if lo.X > hi.X {
			lo, hi = hi, lo
		}
		if lo != geo.NewPos(100, 10) || hi != geo.NewPos(300, 30) {
			t.Errorf("cut to %v..%v, want (100,10)..(300,30)", lo, hi)
		}
	}
	gone := &canvas.Line{Position1: geo.NewPos(320, 0), Position2: geo.NewPos(500, 9)}
	if got := clipX([]core.CanvasObject{gone}, vp); len(got) != 0 {
		t.Errorf("a line wholly right of the plot was kept")
	}
}
