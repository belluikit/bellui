package series

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// EffectScatterSeries configures an animated scatter chart with visual effects.
// Each point renders with a pulsing ring effect.
type EffectScatterSeries struct {
	SeriesName string
	// Data as [x, y] pairs stored flat: [x0, y0, x1, y1, ...]
	Data []float64

	XAxisIndex int
	YAxisIndex int

	SymbolSize float32
	Color      color.Color

	// RippleScale controls the size of the ripple ring relative to the symbol (default 2.5).
	RippleScale float32
}

func (s *EffectScatterSeries) SeriesType() Type { return TypeEffectScatter }
func (s *EffectScatterSeries) Name() string     { return s.SeriesName }
func (s *EffectScatterSeries) sealed()          {}

// GetYAxisIndex returns the Y axis index for this series.
func (s *EffectScatterSeries) GetYAxisIndex() int { return s.YAxisIndex }

// DataRange returns the data extent for axis scaling.
func (s *EffectScatterSeries) DataRange() (xMin, xMax, yMin, yMax float64) {
	n := s.PointCount()
	if n == 0 {
		return 0, 0, 0, 0
	}
	xMin, yMin = math.Inf(1), math.Inf(1)
	xMax, yMax = math.Inf(-1), math.Inf(-1)
	for i := 0; i < n; i++ {
		x, y := s.Point(i)
		if x < xMin {
			xMin = x
		}
		if x > xMax {
			xMax = x
		}
		if y < yMin {
			yMin = y
		}
		if y > yMax {
			yMax = y
		}
	}
	return
}

// PointCount returns the number of data points.
func (s *EffectScatterSeries) PointCount() int {
	return len(s.Data) / 2
}

// Point returns the (x, y) values at index i.
func (s *EffectScatterSeries) Point(i int) (float64, float64) {
	idx := i * 2
	if idx+1 >= len(s.Data) {
		return 0, 0
	}
	return s.Data[idx], s.Data[idx+1]
}

func init() {
	RegisterRenderer(TypeEffectScatter, func() Renderer { return &effectScatterRenderer{} })
}

type effectScatterRenderer struct{}

func (r *effectScatterRenderer) Render(ctx RenderContext) []core.CanvasObject {
	es, ok := ctx.Series.(*EffectScatterSeries)
	if !ok || es.PointCount() == 0 {
		return nil
	}

	size := ResolveFloat32(es.SymbolSize, DefaultScatterSize)
	half := size / 2

	rippleScale := ResolveFloat32(es.RippleScale, DefaultRippleScale)

	fillColor := ResolveColor(es.Color, ctx.Color)

	// Ripple color: same hue, lower alpha
	cr, cg, cb, _ := fillColor.RGBA()
	rippleColor := color.NRGBA{R: uint8(cr >> 8), G: uint8(cg >> 8), B: uint8(cb >> 8), A: 60}

	// Use animation progress to vary ripple size (if animating)
	ripplePhase := float32(1.0)
	if ctx.Animate && ctx.AnimProgress < 1 {
		ripplePhase = ctx.AnimProgress
	}

	var objects []core.CanvasObject

	for i := 0; i < es.PointCount(); i++ {
		x, y := es.Point(i)
		pos := ctx.Coord.DataToPixel(x, y)

		// Ripple ring
		rippleR := half * rippleScale * ripplePhase
		if rippleR > half {
			ripple := canvas.NewCircle(color.Transparent)
			ripple.StrokeColor = rippleColor
			ripple.StrokeWidth = 1.5
			ripple.Position1 = geo.NewPos(pos.X-rippleR, pos.Y-rippleR)
			ripple.Position2 = geo.NewPos(pos.X+rippleR, pos.Y+rippleR)
			objects = append(objects, ripple)
		}

		// Core point
		circle := canvas.NewCircle(fillColor)
		circle.Position1 = geo.NewPos(pos.X-half, pos.Y-half)
		circle.Position2 = geo.NewPos(pos.X+half, pos.Y+half)
		objects = append(objects, circle)
	}

	return objects
}
