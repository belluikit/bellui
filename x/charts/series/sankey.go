package series

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// SankeyNode represents a node in a sankey diagram.
type SankeyNode struct {
	Name  string
	Color color.Color
}

// SankeyLink represents a flow between two nodes.
type SankeyLink struct {
	Source, Target int     // indices into SankeySeries.Nodes
	Value          float64 // flow magnitude
}

// SankeySeries configures a sankey flow diagram.
type SankeySeries struct {
	SeriesName string
	Nodes      []SankeyNode
	Links      []SankeyLink
}

func (s *SankeySeries) SeriesType() Type { return TypeSankey }
func (s *SankeySeries) Name() string     { return s.SeriesName }
func (s *SankeySeries) sealed()          {}

func init() {
	RegisterRenderer(TypeSankey, func() Renderer { return &sankeyRenderer{} })
}

type sankeyRenderer struct{}

func (r *sankeyRenderer) Render(ctx RenderContext) []core.CanvasObject {
	ss, ok := ctx.Series.(*SankeySeries)
	if !ok || len(ss.Nodes) == 0 {
		return nil
	}

	vp := ctx.Viewport
	palette := DefaultPalette

	// Simple two-column layout: sources on the left, targets on the right
	// Compute node layers by finding which nodes are only sources vs only targets
	isTarget := make(map[int]bool)
	isSource := make(map[int]bool)
	for _, l := range ss.Links {
		isSource[l.Source] = true
		isTarget[l.Target] = true
	}

	var leftNodes, rightNodes []int
	for i := range ss.Nodes {
		if isSource[i] && !isTarget[i] {
			leftNodes = append(leftNodes, i)
		} else {
			rightNodes = append(rightNodes, i)
		}
	}
	if len(leftNodes) == 0 {
		// Fallback: first half left, second half right
		mid := len(ss.Nodes) / 2
		for i := 0; i < mid; i++ {
			leftNodes = append(leftNodes, i)
		}
		for i := mid; i < len(ss.Nodes); i++ {
			rightNodes = append(rightNodes, i)
		}
	}

	// Compute total flow per node
	nodeFlow := make([]float64, len(ss.Nodes))
	for _, l := range ss.Links {
		nodeFlow[l.Source] += l.Value
		if l.Value > nodeFlow[l.Target] {
			nodeFlow[l.Target] += l.Value
		}
	}
	maxFlow := 0.0
	for _, f := range nodeFlow {
		if f > maxFlow {
			maxFlow = f
		}
	}
	if maxFlow == 0 {
		maxFlow = 1
	}

	nodeW := float32(20)
	gap := float32(8)
	leftX := vp.X + 10
	rightX := vp.X + vp.W - nodeW - 10

	// Position nodes
	type nodePos struct {
		x, y, h float32
	}
	positions := make([]nodePos, len(ss.Nodes))

	layoutColumn := func(nodes []int, x float32) {
		totalH := vp.H - gap*float32(len(nodes)-1)
		totalFlow := 0.0
		for _, i := range nodes {
			totalFlow += nodeFlow[i]
		}
		if totalFlow == 0 {
			totalFlow = 1
		}
		y := vp.Y
		for _, i := range nodes {
			h := float32(nodeFlow[i]/totalFlow) * totalH
			if h < 4 {
				h = 4
			}
			positions[i] = nodePos{x: x, y: y, h: h}
			y += h + gap
		}
	}

	layoutColumn(leftNodes, leftX)
	layoutColumn(rightNodes, rightX)

	var objects []core.CanvasObject

	// Draw links as rectangles (simplified — proper Sankey uses Bezier curves)
	for _, l := range ss.Links {
		src := positions[l.Source]
		tgt := positions[l.Target]
		linkH := float32(l.Value/maxFlow) * (vp.H * 0.3)
		if linkH < 2 {
			linkH = 2
		}

		srcC := palette[l.Source%len(palette)]
		if ss.Nodes[l.Source].Color != nil {
			srcC = ss.Nodes[l.Source].Color
		}
		cr, cg, cb, _ := srcC.RGBA()
		linkColor := color.NRGBA{R: uint8(cr >> 8), G: uint8(cg >> 8), B: uint8(cb >> 8), A: 60}

		// Draw as a filled rectangle from source right edge to target left edge
		rect := canvas.NewRectangle(linkColor)
		srcMidY := src.y + src.h/2 - linkH/2
		tgtMidY := tgt.y + tgt.h/2 - linkH/2
		// Use midpoint Y
		midY := (srcMidY + tgtMidY) / 2

		rect.Move(geo.NewPos(src.x+nodeW, midY))
		rect.Resize(geo.NewSize(tgt.x-src.x-nodeW, linkH))
		objects = append(objects, rect)
	}

	// Draw nodes
	for i, n := range ss.Nodes {
		p := positions[i]
		c := palette[i%len(palette)]
		if n.Color != nil {
			c = n.Color
		}
		rect := canvas.NewRectangle(c)
		rect.CornerRadius = 2
		rect.Move(geo.NewPos(p.x, p.y))
		rect.Resize(geo.NewSize(nodeW, p.h))
		objects = append(objects, rect)

		// Label
		if n.Name != "" {
			label := canvas.NewText(n.Name, color.NRGBA{R: 220, G: 220, B: 220, A: 255})
			label.TextSize = 10
			labelW := float32(len(n.Name))*6 + 4
			if p.x < vp.X+vp.W/2 {
				label.Move(geo.NewPos(p.x+nodeW+4, p.y+p.h/2-6))
			} else {
				label.Move(geo.NewPos(p.x-labelW-4, p.y+p.h/2-6))
			}
			label.Resize(geo.NewSize(labelW, 12))
			objects = append(objects, label)
		}
	}

	return objects
}
