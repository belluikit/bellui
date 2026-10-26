package series

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// RadarIndicator defines one axis of a radar chart.
type RadarIndicator struct {
	Name string
	Max  float64 // maximum value on this axis (0 = auto)
}

// RadarSeries configures a radar/spider chart series.
type RadarSeries struct {
	SeriesName string
	Indicators []RadarIndicator // one per spoke
	Data       [][]float64      // each inner slice is one data series (len must match Indicators)

	// Names for each data series (for legend/tooltip).
	DataNames []string

	// Visual options
	ShowArea bool    // fill the polygon area
	Opacity  float32 // area fill opacity (0-1), default 0.3
}

// SeriesType returns TypeRadar.
func (s *RadarSeries) SeriesType() Type { return TypeRadar }

// Name returns the series display name.
func (s *RadarSeries) Name() string { return s.SeriesName }

func (s *RadarSeries) sealed() {}

func init() {
	RegisterRenderer(TypeRadar, func() Renderer { return &radarRenderer{} })
}

type radarRenderer struct{}

func (r *radarRenderer) Render(ctx RenderContext) []core.CanvasObject {
	rs, ok := ctx.Series.(*RadarSeries)
	if !ok || len(rs.Indicators) == 0 || len(rs.Data) == 0 {
		return nil
	}

	vp := ctx.Viewport
	centerX := vp.X + vp.W/2
	centerY := vp.Y + vp.H/2
	maxR := vp.W / 2
	if vp.H/2 < maxR {
		maxR = vp.H / 2
	}
	maxR *= 0.8 // leave margin for labels

	nAxes := len(rs.Indicators)
	angleStep := 2 * math.Pi / float64(nAxes)

	// Compute max values per indicator
	maxVals := make([]float64, nAxes)
	for i, ind := range rs.Indicators {
		if ind.Max > 0 {
			maxVals[i] = ind.Max
		} else {
			maxVals[i] = 1
			for _, d := range rs.Data {
				if i < len(d) && d[i] > maxVals[i] {
					maxVals[i] = d[i]
				}
			}
		}
	}

	var objects []core.CanvasObject
	gridColor := color.NRGBA{R: 100, G: 100, B: 100, A: 80}
	labelColor := backgroundLabelColor(ctx, color.NRGBA{R: 180, G: 180, B: 180, A: 255})

	// Draw concentric rings (3 levels)
	for level := 1; level <= 3; level++ {
		frac := float32(level) / 3.0
		for i := 0; i < nAxes; i++ {
			a1 := float64(i)*angleStep - math.Pi/2
			a2 := float64((i+1)%nAxes)*angleStep - math.Pi/2
			x1 := centerX + float32(math.Cos(a1))*maxR*frac
			y1 := centerY + float32(math.Sin(a1))*maxR*frac
			x2 := centerX + float32(math.Cos(a2))*maxR*frac
			y2 := centerY + float32(math.Sin(a2))*maxR*frac
			ring := &canvas.Line{
				Position1:   geo.NewPos(x1, y1),
				Position2:   geo.NewPos(x2, y2),
				StrokeColor: gridColor,
				StrokeWidth: 0.5,
			}
			objects = append(objects, ring)
		}
	}

	// Draw spokes and labels
	for i, ind := range rs.Indicators {
		angle := float64(i)*angleStep - math.Pi/2
		ex := centerX + float32(math.Cos(angle))*maxR
		ey := centerY + float32(math.Sin(angle))*maxR

		spoke := &canvas.Line{
			Position1:   geo.NewPos(centerX, centerY),
			Position2:   geo.NewPos(ex, ey),
			StrokeColor: gridColor,
			StrokeWidth: 0.5,
		}
		objects = append(objects, spoke)

		// Label
		lx := centerX + float32(math.Cos(angle))*maxR*1.08
		ly := centerY + float32(math.Sin(angle))*maxR*1.08
		label := canvas.NewText(ind.Name, labelColor)
		label.TextSize = 10
		labelW := float32(len(ind.Name))*6 + 4
		if lx < centerX {
			label.Alignment = bellui.TextAlignTrailing
			label.Move(geo.NewPos(lx-labelW, ly-6))
		} else {
			label.Alignment = bellui.TextAlignLeading
			label.Move(geo.NewPos(lx, ly-6))
		}
		label.Resize(geo.NewSize(labelW, 12))
		objects = append(objects, label)
	}

	// Draw data polygons
	palette := DefaultPalette

	for di, data := range rs.Data {
		if len(data) < nAxes {
			continue
		}
		c := palette[di%len(palette)]

		// Compute polygon points
		pts := make([]geo.Position, nAxes)
		for i := 0; i < nAxes; i++ {
			angle := float64(i)*angleStep - math.Pi/2
			normVal := float32(data[i] / maxVals[i])
			if normVal > 1 {
				normVal = 1
			}
			px := centerX + float32(math.Cos(angle))*maxR*normVal
			py := centerY + float32(math.Sin(angle))*maxR*normVal
			pts[i] = geo.NewPos(px, py)
		}

		// Area fill (approximated with triangles from center to each edge)
		if rs.ShowArea {
			opacity := rs.Opacity
			if opacity <= 0 {
				opacity = 0.3
			}
			cr, cg, cb, _ := c.RGBA()
			fillColor := color.NRGBA{
				R: uint8(cr >> 8), G: uint8(cg >> 8), B: uint8(cb >> 8),
				A: uint8(opacity * 255),
			}
			// Draw filled triangles from center to each edge pair
			for i := 0; i < nAxes; i++ {
				next := (i + 1) % nAxes
				tri := r.fillTriangle(centerX, centerY, pts[i], pts[next], fillColor)
				objects = append(objects, tri...)
			}
		}

		// Outline
		for i := 0; i < nAxes; i++ {
			next := (i + 1) % nAxes
			line := &canvas.Line{
				Position1:   pts[i],
				Position2:   pts[next],
				StrokeColor: c,
				StrokeWidth: 2,
			}
			objects = append(objects, line)
		}

		// Dots at vertices
		for _, pt := range pts {
			dot := canvas.NewCircle(c)
			dot.Position1 = geo.NewPos(pt.X-3, pt.Y-3)
			dot.Position2 = geo.NewPos(pt.X+3, pt.Y+3)
			objects = append(objects, dot)
		}
	}

	return objects
}

// fillTriangle renders a filled triangle using thin horizontal rectangles.
func (r *radarRenderer) fillTriangle(cx, cy float32, p1, p2 geo.Position, c color.Color) []core.CanvasObject {
	// Use line strips from center to the two points to approximate the fill
	// This is a simple approach; for a proper fill we'd use canvas.Raster
	steps := 12
	var objects []core.CanvasObject
	for s := 0; s <= steps; s++ {
		t := float32(s) / float32(steps)
		// Interpolate between p1 and p2
		ex := p1.X + t*(p2.X-p1.X)
		ey := p1.Y + t*(p2.Y-p1.Y)
		line := &canvas.Line{
			Position1:   geo.NewPos(cx, cy),
			Position2:   geo.NewPos(ex, ey),
			StrokeColor: c,
			StrokeWidth: 2,
		}
		objects = append(objects, line)
	}
	return objects
}
