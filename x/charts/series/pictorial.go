package series

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/internal/mathutil"
)

// PictorialSeries configures a pictorial bar chart where bars are replaced
// by repeated symbols or filled shapes.
type PictorialSeries struct {
	SeriesName string
	Data       []float64
	XAxisIndex int
	YAxisIndex int
	Symbol     SymbolType // shape used for the pictorial fill
	SymbolSize float32    // size of each symbol unit (default 12)
	Color      color.Color
}

func (s *PictorialSeries) SeriesType() Type { return TypePictorial }
func (s *PictorialSeries) Name() string     { return s.SeriesName }
func (s *PictorialSeries) sealed()          {}

// GetYAxisIndex returns the Y axis index for this series.
func (s *PictorialSeries) GetYAxisIndex() int { return s.YAxisIndex }

// DataRange returns the data extent for axis scaling. Includes 0 in Y range.
func (s *PictorialSeries) DataRange() (xMin, xMax, yMin, yMax float64) {
	if len(s.Data) == 0 {
		return 0, 0, 0, 0
	}
	xMin, xMax = 0, float64(len(s.Data)-1)
	yMin, yMax = 0, 0
	for _, v := range s.Data {
		if v < yMin {
			yMin = v
		}
		if v > yMax {
			yMax = v
		}
	}
	return
}

func init() {
	RegisterRenderer(TypePictorial, func() Renderer { return &pictorialRenderer{} })
}

type pictorialRenderer struct{}

func (r *pictorialRenderer) Render(ctx RenderContext) []core.CanvasObject {
	ps, ok := ctx.Series.(*PictorialSeries)
	if !ok || len(ps.Data) == 0 {
		return nil
	}

	bounds := ctx.Coord.Bounds()
	n := len(ps.Data)
	categoryW := bounds.W / float32(n)
	symSize := ResolveFloat32(ps.SymbolSize, DefaultPictorialSize)

	fillColor := ResolveColor(ps.Color, ctx.Color)

	baselineY := ctx.Coord.DataToPixel(0, 0).Y

	var objects []core.CanvasObject
	for i, v := range ps.Data {
		pos := ctx.Coord.DataToPixel(float64(i), v)
		barH := baselineY - pos.Y
		if barH < 0 {
			barH = -barH
		}

		if ctx.Animate && ctx.AnimProgress < 1 {
			barH = mathutil.LerpF32(0, barH, ctx.AnimProgress)
		}

		// Fill the bar area with symbols
		cx := pos.X
		numSymbols := int(barH / (symSize + 2))
		if numSymbols < 1 {
			numSymbols = 1
		}

		for j := 0; j < numSymbols; j++ {
			sy := baselineY - float32(j)*(symSize+2) - symSize
			half := symSize / 2

			switch ps.Symbol {
			case SymbolCircle:
				circle := canvas.NewCircle(fillColor)
				circle.Position1 = geo.NewPos(cx-half, sy)
				circle.Position2 = geo.NewPos(cx+half, sy+symSize)
				objects = append(objects, circle)
			default:
				rect := canvas.NewRectangle(fillColor)
				rect.CornerRadius = 2
				rect.Move(geo.NewPos(cx-categoryW*0.3, sy))
				rect.Resize(geo.NewSize(categoryW*0.6, symSize))
				objects = append(objects, rect)
			}
		}
	}

	return objects
}
