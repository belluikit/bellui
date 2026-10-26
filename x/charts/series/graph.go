package series

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// GraphNode represents a node in a graph/network diagram.
type GraphNode struct {
	Name  string
	X, Y  float64 // position (0-1 normalized); if both 0, auto-layout
	Value float64 // size weight
	Color color.Color
}

// GraphEdge represents an edge between two nodes.
type GraphEdge struct {
	Source, Target int     // indices into GraphSeries.Nodes
	Value          float64 // edge weight (controls thickness)
}

// GraphSeries configures a node-link graph diagram.
type GraphSeries struct {
	SeriesName string
	Nodes      []GraphNode
	Edges      []GraphEdge
	NodeSize   float32 // base node radius (default 8)
}

func (s *GraphSeries) SeriesType() Type { return TypeGraph }
func (s *GraphSeries) Name() string     { return s.SeriesName }
func (s *GraphSeries) sealed()          {}

func init() {
	RegisterRenderer(TypeGraph, func() Renderer { return &graphRenderer{} })
}

type graphRenderer struct{}

func (r *graphRenderer) Render(ctx RenderContext) []core.CanvasObject {
	gs, ok := ctx.Series.(*GraphSeries)
	if !ok || len(gs.Nodes) == 0 {
		return nil
	}

	vp := ctx.Viewport
	nodeSize := ResolveFloat32(gs.NodeSize, DefaultGraphNodeSize)

	palette := DefaultPalette

	// Auto-layout: arrange nodes in a circle if no positions set
	positions := make([][2]float32, len(gs.Nodes))
	hasManualPos := false
	for _, n := range gs.Nodes {
		if n.X != 0 || n.Y != 0 {
			hasManualPos = true
			break
		}
	}

	margin := float32(30)
	for i, n := range gs.Nodes {
		if hasManualPos {
			positions[i] = [2]float32{
				vp.X + margin + float32(n.X)*(vp.W-2*margin),
				vp.Y + margin + float32(n.Y)*(vp.H-2*margin),
			}
		} else {
			// Circular layout
			angle := 2 * math.Pi * float64(i) / float64(len(gs.Nodes))
			cx := vp.X + vp.W/2
			cy := vp.Y + vp.H/2
			r := (vp.W/2 - margin) * 0.7
			if (vp.H/2-margin)*0.7 < r {
				r = (vp.H/2 - margin) * 0.7
			}
			positions[i] = [2]float32{
				cx + float32(math.Cos(angle))*r,
				cy + float32(math.Sin(angle))*r,
			}
		}
	}

	var objects []core.CanvasObject

	// Draw edges
	edgeColor := color.NRGBA{R: 120, G: 120, B: 120, A: 100}
	for _, e := range gs.Edges {
		if e.Source >= len(positions) || e.Target >= len(positions) {
			continue
		}
		w := float32(1)
		if e.Value > 0 {
			w = float32(math.Log2(e.Value+1)) + 1
		}
		line := &canvas.Line{
			Position1:   geo.NewPos(positions[e.Source][0], positions[e.Source][1]),
			Position2:   geo.NewPos(positions[e.Target][0], positions[e.Target][1]),
			StrokeColor: edgeColor,
			StrokeWidth: w,
		}
		objects = append(objects, line)
	}

	// Draw nodes
	for i, n := range gs.Nodes {
		px, py := positions[i][0], positions[i][1]
		r := nodeSize
		if n.Value > 0 {
			r = nodeSize + float32(math.Log2(n.Value+1))*2
		}

		c := palette[i%len(palette)]
		if n.Color != nil {
			c = n.Color
		}

		circle := canvas.NewCircle(c)
		circle.Position1 = geo.NewPos(px-r, py-r)
		circle.Position2 = geo.NewPos(px+r, py+r)
		objects = append(objects, circle)

		// Label
		if n.Name != "" {
			label := canvas.NewText(n.Name, backgroundLabelColor(ctx, color.NRGBA{R: 220, G: 220, B: 220, A: 255}))
			label.TextSize = 10
			labelW := float32(len(n.Name))*6 + 4
			label.Alignment = bellui.TextAlignCenter
			label.Move(geo.NewPos(px-labelW/2, py+r+2))
			label.Resize(geo.NewSize(labelW, 12))
			objects = append(objects, label)
		}
	}

	return objects
}
