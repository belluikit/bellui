package series

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/internal/mathutil"
)

// GeoLine represents a directional line between two geographic points.
type GeoLine struct {
	From, To [2]float64 // [lon, lat]
	Value    float64    // optional weight
}

// GeoLinesSeries renders directional lines between geographic points.
// Designed to overlay on a MapSeries.
type GeoLinesSeries struct {
	SeriesName string
	Data       []GeoLine
	Color      color.Color
	Width      float32
	Curved     bool // true = great circle arc, false = straight

	MapCenter [2]float64
	MapZoom   float32
}

func (s *GeoLinesSeries) SeriesType() Type { return TypeGeoLines }
func (s *GeoLinesSeries) Name() string     { return s.SeriesName }
func (s *GeoLinesSeries) sealed()          {}

// SeriesColor returns the colour set for the series, or nil for the palette
// colour (see ColoredSeries).
//
// Since: 1.3
func (s *GeoLinesSeries) SeriesColor() color.Color { return s.Color }

func init() {
	RegisterRenderer(TypeGeoLines, func() Renderer { return &geoLinesRenderer{} })
}

type geoLinesRenderer struct{}

func (r *geoLinesRenderer) Render(ctx RenderContext) []core.CanvasObject {
	gs, ok := ctx.Series.(*GeoLinesSeries)
	if !ok || len(gs.Data) == 0 {
		return nil
	}

	// Use the shared GeoCoord from ctx.Coord if available, otherwise create one
	gc, ok2 := ctx.Coord.(*coord.GeoCoord)
	if !ok2 {
		gc = coord.NewGeoCoord(gs.MapCenter[0], gs.MapCenter[1], gs.MapZoom)
		gc.SetBounds(ctx.Viewport)
	}

	lineWidth := gs.Width
	if lineWidth <= 0 {
		lineWidth = 2
	}

	c := ResolveColor(gs.Color, ctx.Color)

	// Clip bounds
	clipMinX := ctx.Viewport.X
	clipMinY := ctx.Viewport.Y
	clipMaxX := ctx.Viewport.X + ctx.Viewport.W
	clipMaxY := ctx.Viewport.Y + ctx.Viewport.H

	var objects []core.CanvasObject
	for _, gl := range gs.Data {
		from := gc.DataToPixel(gl.From[0], gl.From[1])
		to := gc.DataToPixel(gl.To[0], gl.To[1])

		if gs.Curved {
			objects = append(objects, r.curvedLine(from, to, c, lineWidth, clipMinX, clipMinY, clipMaxX, clipMaxY)...)
		} else {
			// Clip straight line to viewport
			cx1, cy1, cx2, cy2, vis := mathutil.ClipLineToRect(
				from.X, from.Y, to.X, to.Y,
				clipMinX, clipMinY, clipMaxX, clipMaxY,
			)
			if vis {
				line := &canvas.Line{
					Position1:   geo.NewPos(cx1, cy1),
					Position2:   geo.NewPos(cx2, cy2),
					StrokeColor: c,
					StrokeWidth: lineWidth,
				}
				objects = append(objects, line)
			}
		}

		// Arrow head at destination (only if inside viewport)
		if to.X >= clipMinX && to.X <= clipMaxX && to.Y >= clipMinY && to.Y <= clipMaxY {
			objects = append(objects, r.arrowHead(to, from, c, lineWidth)...)
		}
	}

	return objects
}

// curvedLine draws a curved line from p1 to p2 via a control point above the midpoint.
func (r *geoLinesRenderer) curvedLine(p1, p2 geo.Position, c color.Color, w, clipMinX, clipMinY, clipMaxX, clipMaxY float32) []core.CanvasObject {
	// Control point: midpoint raised by distance/4
	mx := (p1.X + p2.X) / 2
	my := (p1.Y + p2.Y) / 2
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	lift := dist / 8 // subtle curve, not too steep at world scale
	ctrl := geo.NewPos(mx, my-lift)

	// Approximate quadratic bezier with line segments
	segments := 16
	var objects []core.CanvasObject
	prev := p1
	for i := 1; i <= segments; i++ {
		t := float32(i) / float32(segments)
		// Quadratic bezier: B(t) = (1-t)²P0 + 2(1-t)tP1 + t²P2
		mt := 1 - t
		x := mt*mt*p1.X + 2*mt*t*ctrl.X + t*t*p2.X
		y := mt*mt*p1.Y + 2*mt*t*ctrl.Y + t*t*p2.Y
		cur := geo.NewPos(x, y)
		// Clip each segment to viewport
		cx1, cy1, cx2, cy2, vis := mathutil.ClipLineToRect(
			prev.X, prev.Y, cur.X, cur.Y,
			clipMinX, clipMinY, clipMaxX, clipMaxY,
		)
		if vis {
			seg := &canvas.Line{
				Position1:   geo.NewPos(cx1, cy1),
				Position2:   geo.NewPos(cx2, cy2),
				StrokeColor: c,
				StrokeWidth: w,
			}
			objects = append(objects, seg)
		}
		prev = cur
	}
	return objects
}

// arrowHead draws a small arrow at the destination point.
func (r *geoLinesRenderer) arrowHead(tip, from geo.Position, c color.Color, w float32) []core.CanvasObject {
	dx := tip.X - from.X
	dy := tip.Y - from.Y
	length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if length == 0 {
		return nil
	}

	// Normalize direction
	ux := dx / length
	uy := dy / length
	arrowLen := w * 4

	// Two arrow wing points
	wing1 := geo.NewPos(tip.X-ux*arrowLen+uy*arrowLen/2, tip.Y-uy*arrowLen-ux*arrowLen/2)
	wing2 := geo.NewPos(tip.X-ux*arrowLen-uy*arrowLen/2, tip.Y-uy*arrowLen+ux*arrowLen/2)

	l1 := &canvas.Line{Position1: tip, Position2: wing1, StrokeColor: c, StrokeWidth: w}
	l2 := &canvas.Line{Position1: tip, Position2: wing2, StrokeColor: c, StrokeWidth: w}
	return []core.CanvasObject{l1, l2}
}
