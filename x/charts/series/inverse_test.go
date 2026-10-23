package series

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/x/charts/axis"
	"github.com/belluikit/bellui/x/charts/coord"
)

// An Inverse Y axis (axis.Config.Inverse, applied through
// Cartesian2D.SetInvert) puts the axis minimum at the top of the plot, so a
// positive value grows downwards from the baseline. The renderers below
// assumed it always grows upwards.

var invPlot = coord.Rect{X: 0, Y: 0, W: 300, H: 200}

func inverseYCoord(n int, lo, hi float64) *coord.Cartesian2D {
	xs := axis.NewCategoryScale(make([]string, n))
	xs.SetRange(0, float64(n-1))
	ys := axis.NewLinearScale(5, &lo, &hi)
	ys.SetRange(lo, hi)
	cs := coord.NewCartesian2D(xs, ys)
	cs.SetBounds(invPlot)
	cs.SetInvert(false, true)
	return cs
}

func rects(objs []core.CanvasObject) []*canvas.Rectangle {
	var out []*canvas.Rectangle
	for _, o := range objs {
		if r, ok := o.(*canvas.Rectangle); ok {
			out = append(out, r)
		}
	}
	return out
}

func TestCandlestick_InverseY(t *testing.T) {
	cs := inverseYCoord(1, 0, 100)
	objs := NewRenderer(TypeCandlestick).Render(RenderContext{
		Series:   &CandlestickSeries{Data: []CandlestickData{{Open: 20, Close: 60, High: 80, Low: 10}}},
		Coord:    cs,
		Viewport: invPlot,
		Color:    color.White,
	})
	open, cls := cs.DataToPixel(0, 20).Y, cs.DataToPixel(0, 60).Y
	high, low := cs.DataToPixel(0, 80).Y, cs.DataToPixel(0, 10).Y

	bodies := rects(objs)
	if len(bodies) != 1 {
		t.Fatalf("drew %d bodies, want 1", len(bodies))
	}
	body := bodies[0]
	if body.Position().Y != open || body.Size().Height != cls-open {
		t.Errorf("body spans y %v..%v, want open..close %v..%v",
			body.Position().Y, body.Position().Y+body.Size().Height, open, cls)
	}

	// One wick from the body to the high, one from the body to the low.
	var toHigh, toLow bool
	for _, o := range objs {
		l, ok := o.(*canvas.Line)
		if !ok {
			continue
		}
		y1, y2 := l.Position1.Y, l.Position2.Y
		if (y1 == cls && y2 == high) || (y1 == high && y2 == cls) {
			toHigh = true
		}
		if (y1 == open && y2 == low) || (y1 == low && y2 == open) {
			toLow = true
		}
	}
	if !toHigh || !toLow {
		t.Errorf("wicks: close-to-high=%v open-to-low=%v, want both", toHigh, toLow)
	}
}

func TestPictorial_InverseY(t *testing.T) {
	cs := inverseYCoord(1, 0, 100)
	objs := NewRenderer(TypePictorial).Render(RenderContext{
		Series:   &PictorialSeries{Data: []float64{50}, SymbolSize: 10},
		Coord:    cs,
		Viewport: invPlot,
		Color:    color.White,
	})
	base, top := cs.DataToPixel(0, 0).Y, cs.DataToPixel(0, 50).Y
	syms := rects(objs)
	if len(syms) < 2 {
		t.Fatalf("drew %d symbols, want a bar of them", len(syms))
	}
	for _, s := range syms {
		if y := s.Position().Y; y < base || y+s.Size().Height > top {
			t.Errorf("symbol at y %v..%v, want it between the baseline %v and the value %v",
				y, y+s.Size().Height, base, top)
		}
	}
}

func TestBar_InverseYAnimation(t *testing.T) {
	cs := inverseYCoord(1, 0, 100)
	objs := NewRenderer(TypeBar).Render(RenderContext{
		Series:       &BarSeries{Data: []float64{50}},
		Coord:        cs,
		Viewport:     invPlot,
		Color:        color.White,
		Animate:      true,
		AnimProgress: 0.5,
	})
	base := cs.DataToPixel(0, 0).Y
	bars := rects(objs)
	if len(bars) != 1 {
		t.Fatalf("drew %d bars, want 1", len(bars))
	}
	if y := bars[0].Position().Y; y != base {
		t.Errorf("half-grown bar starts at y=%v, want the baseline %v (it grows down on an inverse axis)", y, base)
	}
	if h := bars[0].Size().Height; h <= 0 {
		t.Errorf("half-grown bar height %v, want > 0", h)
	}
}
