package series

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// SunburstNode represents a node in a sunburst hierarchy.
type SunburstNode struct {
	Name     string
	Value    float64
	Color    color.Color
	Children []SunburstNode
}

// SunburstSeries configures a sunburst chart series.
type SunburstSeries struct {
	SeriesName string
	Data       []SunburstNode
}

func (s *SunburstSeries) SeriesType() Type { return TypeSunburst }
func (s *SunburstSeries) Name() string     { return s.SeriesName }
func (s *SunburstSeries) sealed()          {}

func init() {
	RegisterRenderer(TypeSunburst, func() Renderer { return &sunburstRenderer{} })
}

type sunburstRenderer struct{}

func (r *sunburstRenderer) Render(ctx RenderContext) []core.CanvasObject {
	ss, ok := ctx.Series.(*SunburstSeries)
	if !ok || len(ss.Data) == 0 {
		return nil
	}

	vp := ctx.Viewport
	centerX := vp.X + vp.W/2
	centerY := vp.Y + vp.H/2
	maxR := vp.W / 2
	if vp.H/2 < maxR {
		maxR = vp.H / 2
	}
	maxR *= 0.9

	palette := DefaultPalette

	// Count max depth for ring sizing
	maxDepth := r.maxDepth(ss.Data, 0)
	if maxDepth == 0 {
		maxDepth = 1
	}
	ringWidth := maxR / float32(maxDepth+1)

	var objects []core.CanvasObject
	r.renderRing(ss.Data, 0, 360, 0, centerX, centerY, ringWidth, palette, &objects)
	return objects
}

func (r *sunburstRenderer) renderRing(nodes []SunburstNode, startAngle, endAngle float32, depth int,
	cx, cy, ringW float32, palette []color.Color, objects *[]core.CanvasObject,
) {
	total := 0.0
	for _, n := range nodes {
		total += sunburstNodeValue(n)
	}
	if total == 0 {
		return
	}

	innerR := ringW * float32(depth)
	outerR := ringW * float32(depth+1)
	cutout := innerR / outerR
	if depth == 0 {
		cutout = 0
	}

	angle := startAngle
	for i, n := range nodes {
		frac := float32(sunburstNodeValue(n) / total)
		sweep := (endAngle - startAngle) * frac

		c := palette[i%len(palette)]
		if n.Color != nil {
			c = n.Color
		}
		// Lighten for deeper levels
		if depth > 0 {
			cr, cg, cb, ca := c.RGBA()
			factor := float32(0.7 + 0.1*float32(depth))
			if factor > 1 {
				factor = 1
			}
			c = color.NRGBA{
				R: uint8(float32(cr>>8)*factor + 255*(1-factor)),
				G: uint8(float32(cg>>8)*factor + 255*(1-factor)),
				B: uint8(float32(cb>>8)*factor + 255*(1-factor)),
				A: uint8(ca >> 8),
			}
		}

		arc := canvas.NewArc(angle, angle+sweep, cutout, c)
		arc.Move(geo.NewPos(cx-outerR, cy-outerR))
		arc.Resize(geo.NewSize(outerR*2, outerR*2))
		*objects = append(*objects, arc)

		// Label on larger segments
		if sweep > 20 && n.Name != "" {
			midAngle := angle + sweep/2
			labelR := (innerR + outerR) / 2
			rad := float64(midAngle-90) * math.Pi / 180
			lx := cx + float32(math.Cos(rad))*labelR
			ly := cy + float32(math.Sin(rad))*labelR
			label := canvas.NewText(n.Name, labelColorOn(c))
			label.TextSize = 9
			label.Alignment = bellui.TextAlignCenter
			labelW := float32(len(n.Name))*6 + 4
			label.Move(geo.NewPos(lx-labelW/2, ly-5))
			label.Resize(geo.NewSize(labelW, 10))
			*objects = append(*objects, label)
		}

		// Recurse into children
		if len(n.Children) > 0 {
			r.renderRing(n.Children, angle, angle+sweep, depth+1, cx, cy, ringW, palette, objects)
		}

		angle += sweep
	}
}

func (r *sunburstRenderer) maxDepth(nodes []SunburstNode, current int) int {
	max := current
	for _, n := range nodes {
		if len(n.Children) > 0 {
			d := r.maxDepth(n.Children, current+1)
			if d > max {
				max = d
			}
		}
	}
	return max
}

func sunburstNodeValue(n SunburstNode) float64 {
	if len(n.Children) == 0 {
		return n.Value
	}
	sum := 0.0
	for _, c := range n.Children {
		sum += sunburstNodeValue(c)
	}
	return sum
}
