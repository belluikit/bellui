package series

import (
	"image/color"
	"sort"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// TreemapNode represents a node in a treemap hierarchy.
type TreemapNode struct {
	Name     string
	Value    float64
	Color    color.Color
	Children []TreemapNode
}

// TreemapSeries configures a treemap chart series.
type TreemapSeries struct {
	SeriesName string
	Data       []TreemapNode // top-level nodes
}

func (s *TreemapSeries) SeriesType() Type { return TypeTreemap }
func (s *TreemapSeries) Name() string     { return s.SeriesName }
func (s *TreemapSeries) sealed()          {}

func init() {
	RegisterRenderer(TypeTreemap, func() Renderer { return &treemapRenderer{} })
}

type treemapRenderer struct{}

func (r *treemapRenderer) Render(ctx RenderContext) []core.CanvasObject {
	ts, ok := ctx.Series.(*TreemapSeries)
	if !ok || len(ts.Data) == 0 {
		return nil
	}

	vp := ctx.Viewport
	palette := DefaultPalette

	var objects []core.CanvasObject
	r.squarify(ts.Data, vp.X, vp.Y, vp.W, vp.H, palette, 0, &objects)
	return objects
}

// squarify implements a simple slice-and-dice treemap layout.
func (r *treemapRenderer) squarify(nodes []TreemapNode, x, y, w, h float32, palette []color.Color, depth int, objects *[]core.CanvasObject) {
	total := 0.0
	for _, n := range nodes {
		total += nodeValue(n)
	}
	if total == 0 || w < 1 || h < 1 {
		return
	}

	// Sort by value descending for better layout
	sorted := make([]TreemapNode, len(nodes))
	copy(sorted, nodes)
	sort.Slice(sorted, func(i, j int) bool {
		return nodeValue(sorted[i]) > nodeValue(sorted[j])
	})

	horizontal := w >= h
	pos := float32(0)
	dim := w
	if !horizontal {
		dim = h
	}

	gap := float32(1)

	for i, node := range sorted {
		frac := float32(nodeValue(node) / total)
		size := dim * frac

		var rx, ry, rw, rh float32
		if horizontal {
			rx = x + pos
			ry = y
			rw = size - gap
			rh = h - gap
		} else {
			rx = x
			ry = y + pos
			rw = w - gap
			rh = size - gap
		}

		if rw < 1 {
			rw = 1
		}
		if rh < 1 {
			rh = 1
		}

		c := palette[i%len(palette)]
		if node.Color != nil {
			c = node.Color
		}
		// Darken for deeper levels
		if depth > 0 {
			cr, cg, cb, ca := c.RGBA()
			factor := float32(0.7)
			c = color.NRGBA{
				R: uint8(float32(cr>>8) * factor),
				G: uint8(float32(cg>>8) * factor),
				B: uint8(float32(cb>>8) * factor),
				A: uint8(ca >> 8),
			}
		}

		rect := canvas.NewRectangle(c)
		rect.StrokeColor = color.NRGBA{R: 30, G: 30, B: 30, A: 200}
		rect.StrokeWidth = 1
		rect.Move(geo.NewPos(rx, ry))
		rect.Resize(geo.NewSize(rw, rh))
		*objects = append(*objects, rect)

		// Label if there's enough space
		if rw > 30 && rh > 14 && node.Name != "" {
			label := canvas.NewText(node.Name, labelColorOn(c))
			label.TextSize = 10
			label.Move(geo.NewPos(rx+3, ry+2))
			label.Resize(geo.NewSize(rw-6, 12))
			*objects = append(*objects, label)
		}

		// Recurse into children
		if len(node.Children) > 0 {
			childY := ry
			childH := rh
			if rh > 16 {
				childY += 14
				childH -= 14
			}
			r.squarify(node.Children, rx+1, childY, rw-2, childH, palette, depth+1, objects)
		}

		pos += size
	}
}

func nodeValue(n TreemapNode) float64 {
	if len(n.Children) == 0 {
		return n.Value
	}
	sum := 0.0
	for _, c := range n.Children {
		sum += nodeValue(c)
	}
	return sum
}
