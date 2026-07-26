package series

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/coord"
)

// ScatterSeries configures a scatter chart series.
type ScatterSeries struct {
	SeriesName string
	// Data is pairs of [x, y] values, stored flat: [x0, y0, x1, y1, ...]
	Data []float64

	XAxisIndex int
	YAxisIndex int

	Symbol     SymbolType
	SymbolSize float32

	Color color.Color
}

// SeriesType returns TypeScatter.
func (s *ScatterSeries) SeriesType() Type { return TypeScatter }

// Name returns the series display name.
func (s *ScatterSeries) Name() string { return s.SeriesName }

func (s *ScatterSeries) sealed() {}

// GetYAxisIndex returns the Y axis index for this series.
func (s *ScatterSeries) GetYAxisIndex() int { return s.YAxisIndex }

// RegisterHitAreas registers point-based hit areas for the scatter series.
func (s *ScatterSeries) RegisterHitAreas(collector HitAreaCollector, seriesIndex int, coordSys coord.System, plotRect coord.Rect) {
	n := s.PointCount()
	if n == 0 {
		return
	}
	points := make([]HitPoint, n)
	for i := 0; i < n; i++ {
		x, y := s.Point(i)
		pos := coordSys.DataToPixel(x, y)
		points[i] = HitPoint{
			DataIndex: i,
			PixelPos:  pos,
			Value:     HitValue{X: x, Y: y},
		}
	}
	collector.AddHitPoints(seriesIndex, s.SeriesName, TypeScatter, points)
}

// AppendValues appends x,y pairs to the scatter series (x0, y0, x1, y1, ...).
func (s *ScatterSeries) AppendValues(values ...float64) {
	s.Data = append(s.Data, values...)
}

// DataRange returns the data extent for axis scaling.
func (s *ScatterSeries) DataRange() (xMin, xMax, yMin, yMax float64) {
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
func (s *ScatterSeries) PointCount() int {
	return len(s.Data) / 2
}

// Point returns the x, y values at index i.
func (s *ScatterSeries) Point(i int) (float64, float64) {
	idx := i * 2
	if idx+1 >= len(s.Data) {
		return 0, 0
	}
	return s.Data[idx], s.Data[idx+1]
}

func init() {
	RegisterRenderer(TypeScatter, func() Renderer { return &scatterRenderer{} })
}

type scatterRenderer struct{}

func (r *scatterRenderer) Render(ctx RenderContext) []core.CanvasObject {
	ss, ok := ctx.Series.(*ScatterSeries)
	if !ok || ss.PointCount() == 0 {
		return nil
	}

	sym := ss.Symbol
	if sym == SymbolNone {
		sym = SymbolCircle
	}
	size := ResolveFloat32(ss.SymbolSize, DefaultScatterSize)
	half := size / 2

	fillColor := ResolveColor(ss.Color, ctx.Color)

	n := ss.PointCount()
	objects := make([]core.CanvasObject, 0, n)

	for i := 0; i < n; i++ {
		x, y := ss.Point(i)
		pos := ctx.Coord.DataToPixel(x, y)

		obj := CreatePooledSymbol(ctx.Pool, sym, size, pos, fillColor)
		if obj == nil {
			continue
		}

		if ctx.Animate && ctx.AnimProgress < 1 {
			// Fade in via alpha -- approximate by scaling size
			scaledHalf := half * ctx.AnimProgress
			switch o := obj.(type) {
			case *canvas.Circle:
				o.Position1 = geo.NewPos(pos.X-scaledHalf, pos.Y-scaledHalf)
				o.Position2 = geo.NewPos(pos.X+scaledHalf, pos.Y+scaledHalf)
			case *canvas.Rectangle:
				scaledSize := size * ctx.AnimProgress
				o.Move(geo.NewPos(pos.X-scaledHalf, pos.Y-scaledHalf))
				o.Resize(geo.NewSize(scaledSize, scaledSize))
			}
		}

		objects = append(objects, obj)
	}

	return objects
}
