package series

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/x/charts/coord"
)

// FunnelSeries.Sort orders the stages; it was never read (#343). Empty sorts
// descending, as in ECharts, "none" keeps the data order, and a sorted stage
// keeps its own colour.
func TestFunnel_Sort(t *testing.T) {
	red := color.NRGBA{R: 255, A: 255}
	data := []FunnelData{{Name: "mid", Value: 50}, {Name: "top", Value: 90, Color: red}, {Name: "low", Value: 10}}
	order := func(sortBy string) (names []string, topColor color.Color) {
		fs := &FunnelSeries{SeriesName: "f", Data: data, Sort: sortBy}
		ctx := RenderContext{Series: fs, Viewport: coord.Rect{W: 300, H: 300}, Color: color.White}
		var first *canvas.Rectangle
		for _, o := range NewRenderer(TypeFunnel).Render(ctx) {
			switch v := o.(type) {
			case *canvas.Text:
				names = append(names, v.Text)
			case *canvas.Rectangle:
				if first == nil {
					first = v
				}
			}
		}
		return names, first.FillColor
	}
	for sortBy, want := range map[string][]string{
		"":           {"top", "mid", "low"},
		"none":       {"mid", "top", "low"},
		"descending": {"top", "mid", "low"},
		"ascending":  {"low", "mid", "top"},
	} {
		got, _ := order(sortBy)
		if len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
			t.Errorf("Sort %q: stages %v, want %v", sortBy, got, want)
		}
	}
	if _, top := order("descending"); top != red {
		t.Errorf("sorted to the top, the red stage is drawn %v", top)
	}
}
