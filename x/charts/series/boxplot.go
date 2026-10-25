package series

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// BoxplotData holds the five-number summary for one box.
type BoxplotData struct {
	Min, Q1, Median, Q3, Max float64
}

// BoxplotSeries configures a box-and-whisker chart series.
type BoxplotSeries struct {
	SeriesName string
	Data       []BoxplotData
	XAxisIndex int
	YAxisIndex int
	BoxWidth   float32     // fraction of category width (0 = auto 0.5)
	Color      color.Color // box fill color
}

func (s *BoxplotSeries) SeriesType() Type { return TypeBoxplot }
func (s *BoxplotSeries) Name() string     { return s.SeriesName }
func (s *BoxplotSeries) sealed()          {}

// SeriesColor returns the colour set for the series, or nil for the palette
// colour (see ColoredSeries).
//
// Since: 1.3
func (s *BoxplotSeries) SeriesColor() color.Color { return s.Color }

// GetYAxisIndex returns the Y axis index for this series.
func (s *BoxplotSeries) GetYAxisIndex() int { return s.YAxisIndex }

// DataRange returns the data extent for axis scaling.
func (s *BoxplotSeries) DataRange() (xMin, xMax, yMin, yMax float64) {
	if len(s.Data) == 0 {
		return 0, 0, 0, 0
	}
	xMin, xMax = 0, float64(len(s.Data)-1)
	yMin, yMax = math.Inf(1), math.Inf(-1)
	for _, d := range s.Data {
		if d.Min < yMin {
			yMin = d.Min
		}
		if d.Max > yMax {
			yMax = d.Max
		}
	}
	return
}

func init() {
	RegisterRenderer(TypeBoxplot, func() Renderer { return &boxplotRenderer{} })
}

type boxplotRenderer struct{}

func (r *boxplotRenderer) Render(ctx RenderContext) []core.CanvasObject {
	bs, ok := ctx.Series.(*BoxplotSeries)
	if !ok || len(bs.Data) == 0 {
		return nil
	}

	n := len(bs.Data)

	boxWidth := ResolveFloat32(bs.BoxWidth, DefaultBoxWidth)
	categoryW := categoryWidthOf(ctx.Coord, n)
	bw := categoryW * boxWidth

	fillColor := ResolveColor(bs.Color, ctx.Color)
	borderColor := fillColor
	whiskerColor := color.NRGBA{R: 200, G: 200, B: 200, A: 255}

	var objects []core.CanvasObject
	for i, d := range bs.Data {
		cx := ctx.Coord.DataToPixel(float64(i), 0).X

		minPos := ctx.Coord.DataToPixel(float64(i), d.Min)
		q1Pos := ctx.Coord.DataToPixel(float64(i), d.Q1)
		medPos := ctx.Coord.DataToPixel(float64(i), d.Median)
		q3Pos := ctx.Coord.DataToPixel(float64(i), d.Q3)
		maxPos := ctx.Coord.DataToPixel(float64(i), d.Max)

		// IQR box (Q1 to Q3)
		boxTop := q3Pos.Y
		boxBot := q1Pos.Y
		if boxTop > boxBot {
			boxTop, boxBot = boxBot, boxTop
		}
		box := canvas.NewRectangle(fillColor)
		box.StrokeColor = borderColor
		box.StrokeWidth = 1
		box.Move(geo.NewPos(cx-bw/2, boxTop))
		box.Resize(geo.NewSize(bw, boxBot-boxTop))
		objects = append(objects, box)

		// Median line
		median := &canvas.Line{
			Position1:   geo.NewPos(cx-bw/2, medPos.Y),
			Position2:   geo.NewPos(cx+bw/2, medPos.Y),
			StrokeColor: color.NRGBA{R: 255, G: 255, B: 255, A: 255},
			StrokeWidth: 2,
		}
		objects = append(objects, median)

		// Upper whisker (Q3 to Max)
		upperWhisker := &canvas.Line{
			Position1:   geo.NewPos(cx, boxTop),
			Position2:   geo.NewPos(cx, maxPos.Y),
			StrokeColor: whiskerColor,
			StrokeWidth: 1,
		}
		objects = append(objects, upperWhisker)
		upperCap := &canvas.Line{
			Position1:   geo.NewPos(cx-bw/4, maxPos.Y),
			Position2:   geo.NewPos(cx+bw/4, maxPos.Y),
			StrokeColor: whiskerColor,
			StrokeWidth: 1,
		}
		objects = append(objects, upperCap)

		// Lower whisker (Q1 to Min)
		lowerWhisker := &canvas.Line{
			Position1:   geo.NewPos(cx, boxBot),
			Position2:   geo.NewPos(cx, minPos.Y),
			StrokeColor: whiskerColor,
			StrokeWidth: 1,
		}
		objects = append(objects, lowerWhisker)
		lowerCap := &canvas.Line{
			Position1:   geo.NewPos(cx-bw/4, minPos.Y),
			Position2:   geo.NewPos(cx+bw/4, minPos.Y),
			StrokeColor: whiskerColor,
			StrokeWidth: 1,
		}
		objects = append(objects, lowerCap)
	}

	return objects
}
