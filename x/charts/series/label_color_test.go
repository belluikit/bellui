package series

import (
	"image/color"
	"testing"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/x/charts/coord"
)

// textColors maps each label text to the colour it is drawn in.
func textColors(objs []any) map[string]color.Color {
	m := map[string]color.Color{}
	for _, o := range objs {
		if t, ok := o.(*canvas.Text); ok {
			m[t.Text] = t.Color
		}
	}
	return m
}

func renderAll(s Series, ctx RenderContext) []any {
	ctx.Series = s
	var out []any
	for _, o := range NewRenderer(s.SeriesType()).Render(ctx) {
		out = append(out, o)
	}
	return out
}

// Labels on the plot background take RenderContext.TextColor, the theme's
// foreground; they were fixed light greys, nearly invisible on a light theme.
// Labels on a coloured tile are black or white, whichever contrasts with the
// tile; white on amber was faint (#343).
func TestSeriesLabels_FollowTheThemeAndTheirTile(t *testing.T) {
	dark := color.NRGBA{R: 30, G: 30, B: 30, A: 255}
	amber := color.NRGBA{R: 250, G: 200, B: 88, A: 255}
	navy := color.NRGBA{R: 20, G: 30, B: 90, A: 255}
	ctx := RenderContext{Viewport: coord.Rect{W: 400, H: 300}, Color: color.White, TextColor: dark}

	background := map[string]Series{
		"62.0":  &GaugeSeries{SeriesName: "g", Value: 62, ShowValue: true},
		"leaf":  &TreeSeries{SeriesName: "t", Data: TreeNode{Name: "root", Children: []TreeNode{{Name: "leaf"}}}},
		"A":     &GraphSeries{SeriesName: "gr", Nodes: []GraphNode{{Name: "A"}, {Name: "B"}}, Edges: []GraphEdge{{Source: 0, Target: 1}}},
		"Out":   &SankeySeries{SeriesName: "s", Nodes: []SankeyNode{{Name: "In"}, {Name: "Out"}}, Links: []SankeyLink{{Source: 0, Target: 1, Value: 5}}},
		"depth": &ParallelSeries{SeriesName: "p", Axes: []ParallelAxis{{Name: "depth"}, {Name: "width"}}, Data: [][]float64{{1, 2}, {3, 4}}},
		"speed": &RadarSeries{SeriesName: "r", Indicators: []RadarIndicator{{Name: "speed", Max: 10}, {Name: "power", Max: 10}, {Name: "range", Max: 10}}, Data: [][]float64{{5, 6, 7}}},
	}
	for label, s := range background {
		if got := textColors(renderAll(s, ctx))[label]; got != dark {
			t.Errorf("%T label %q is %v, want the context's text colour %v", s, label, got, dark)
		}
	}

	tiles := map[string]Series{
		"Funnel": &FunnelSeries{SeriesName: "f", Data: []FunnelData{{Name: "Funnel", Value: 1, Color: amber}, {Name: "Deep", Value: 1, Color: navy}}},
		"Ring":   &SunburstSeries{SeriesName: "sb", Data: []SunburstNode{{Name: "Ring", Value: 1, Color: amber}, {Name: "Deep", Value: 1, Color: navy}}},
		"Tile":   &TreemapSeries{SeriesName: "t", Data: []TreemapNode{{Name: "Tile", Value: 3, Color: amber}, {Name: "Deep", Value: 3, Color: navy}}},
	}
	for label, s := range tiles {
		colors := textColors(renderAll(s, ctx))
		if got := colors[label]; got != color.Black {
			t.Errorf("%T label %q on amber is %v, want black", s, label, got)
		}
		if got := colors["Deep"]; got != color.White {
			t.Errorf("%T label \"Deep\" on navy is %v, want white", s, got)
		}
	}
}
