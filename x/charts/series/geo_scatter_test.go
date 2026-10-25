package series

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/x/charts/coord"
)

// GeoScatterSeries.Symbol picks the marker; it was never read and every point
// drew a circle (#343). No symbol still means a circle.
func TestGeoScatter_DrawsItsSymbol(t *testing.T) {
	viewport := coord.Rect{X: 0, Y: 0, W: 400, H: 300}
	gc := coord.NewGeoCoord(0, 0, 1)
	gc.SetBounds(viewport)
	render := func(sym SymbolType) any {
		gs := &GeoScatterSeries{SeriesName: "g", Data: []float64{0, 0}, Symbol: sym}
		objs := NewRenderer(TypeGeoScatter).Render(RenderContext{Series: gs, Coord: gc, Viewport: viewport, Color: color.White})
		if len(objs) != 1 {
			t.Fatalf("symbol %d: %d objects, want 1", sym, len(objs))
		}
		return objs[0]
	}

	if _, ok := render(SymbolRect).(*canvas.Rectangle); !ok {
		t.Errorf("SymbolRect drew a %T, want a rectangle", render(SymbolRect))
	}
	for _, sym := range []SymbolType{SymbolNone, SymbolCircle} {
		if _, ok := render(sym).(*canvas.Circle); !ok {
			t.Errorf("symbol %d drew a %T, want a circle", sym, render(sym))
		}
	}
}
