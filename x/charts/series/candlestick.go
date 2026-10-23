package series

import (
	"fmt"
	"image/color"
	"math"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/internal/mathutil"
)

// CandlestickData holds OHLC data for one candlestick.
type CandlestickData struct {
	Open, High, Low, Close float64
}

// CandlestickSeries configures a candlestick/OHLC chart series.
type CandlestickSeries struct {
	SeriesName string
	Data       []CandlestickData

	XAxisIndex int
	YAxisIndex int

	// BarWidth as fraction of category spacing (0-1). 0 = auto.
	BarWidth float32

	// Colors for up (close >= open) and down (close < open) candles.
	UpColor   color.Color
	DownColor color.Color
}

// SeriesType returns TypeCandlestick.
func (s *CandlestickSeries) SeriesType() Type { return TypeCandlestick }

// Name returns the series display name.
func (s *CandlestickSeries) Name() string { return s.SeriesName }

func (s *CandlestickSeries) sealed() {}

// GetYAxisIndex returns the Y axis index for this series.
func (s *CandlestickSeries) GetYAxisIndex() int { return s.YAxisIndex }

// RegisterHitAreas registers rectangle-based hit areas covering the low-to-high range of each candle.
func (s *CandlestickSeries) RegisterHitAreas(collector HitAreaCollector, seriesIndex int, coordSys coord.System, plotRect coord.Rect) {
	n := len(s.Data)
	if n == 0 {
		return
	}
	barWidth := ResolveFloat32(s.BarWidth, DefaultBarWidth)
	categoryWidth := plotRect.W / float32(n)
	bw := categoryWidth * barWidth

	bars := make([]HitBar, n)
	for i, d := range s.Data {
		highPos := coordSys.DataToPixel(float64(i), d.High)
		lowPos := coordSys.DataToPixel(float64(i), d.Low)
		cx := highPos.X
		top := highPos.Y
		bot := lowPos.Y
		if top > bot {
			top, bot = bot, top
		}
		bars[i] = HitBar{
			DataIndex: i,
			Rect:      [4]float32{cx - bw/2, top, bw, bot - top},
			PixelPos:  geo.NewPos(cx, (top+bot)/2),
			Value:     HitValue{X: float64(i), Y: d.Close, Label: fmt.Sprintf("O:%.0f H:%.0f L:%.0f C:%.0f", d.Open, d.High, d.Low, d.Close)},
		}
	}
	collector.AddHitBars(seriesIndex, s.SeriesName, TypeCandlestick, bars)
}

// AppendCandles appends candlestick data points.
func (s *CandlestickSeries) AppendCandles(candles ...CandlestickData) {
	s.Data = append(s.Data, candles...)
}

// DataRange returns the data extent for axis scaling.
func (s *CandlestickSeries) DataRange() (xMin, xMax, yMin, yMax float64) {
	if len(s.Data) == 0 {
		return 0, 0, 0, 0
	}
	xMin, xMax = 0, float64(len(s.Data)-1)
	yMin, yMax = math.Inf(1), math.Inf(-1)
	for _, d := range s.Data {
		if d.Low < yMin {
			yMin = d.Low
		}
		if d.High > yMax {
			yMax = d.High
		}
	}
	return
}

func init() {
	RegisterRenderer(TypeCandlestick, func() Renderer { return &candlestickRenderer{} })
}

type candlestickRenderer struct{}

func (r *candlestickRenderer) Render(ctx RenderContext) []core.CanvasObject {
	cs, ok := ctx.Series.(*CandlestickSeries)
	if !ok || len(cs.Data) == 0 {
		return nil
	}

	bounds := ctx.Coord.Bounds()
	n := len(cs.Data)

	barWidth := ResolveFloat32(cs.BarWidth, DefaultBarWidth)
	categoryWidth := bounds.W / float32(n)
	bw := categoryWidth * barWidth
	wickWidth := float32(1)

	upColor := cs.UpColor
	if upColor == nil {
		upColor = color.NRGBA{R: 38, G: 186, B: 121, A: 255} // green
	}
	downColor := cs.DownColor
	if downColor == nil {
		downColor = color.NRGBA{R: 239, G: 83, B: 80, A: 255} // red
	}

	clipMinX := ctx.Viewport.X
	clipMaxX := ctx.Viewport.X + ctx.Viewport.W

	objects := make([]core.CanvasObject, 0, n*3)
	for i, d := range cs.Data {
		centerPos := ctx.Coord.DataToPixel(float64(i), 0)
		cx := centerPos.X

		// Skip if outside viewport
		if cx+bw/2 < clipMinX || cx-bw/2 > clipMaxX {
			continue
		}

		openPos := ctx.Coord.DataToPixel(float64(i), d.Open)
		closePos := ctx.Coord.DataToPixel(float64(i), d.Close)
		highPos := ctx.Coord.DataToPixel(float64(i), d.High)
		lowPos := ctx.Coord.DataToPixel(float64(i), d.Low)

		isUp := d.Close >= d.Open
		c := downColor
		if isUp {
			c = upColor
		}

		// Body rectangle (between open and close). Take the top and bottom
		// from the pixels, not from the candle's direction: on an Inverse Y
		// axis the higher price is the lower pixel.
		bodyTop := min(openPos.Y, closePos.Y)
		bodyBot := max(openPos.Y, closePos.Y)
		bodyH := bodyBot - bodyTop
		if bodyH < 1 {
			bodyH = 1
		}

		body := canvas.NewRectangle(c)
		body.Move(geo.NewPos(cx-bw/2, bodyTop))
		body.Resize(geo.NewSize(bw, bodyH))
		objects = append(objects, body)

		// Wicks run from the high and from the low to the body edge nearest
		// each, which is the top edge for the high unless the axis is inverse.
		// A high or low that does not reach beyond the body draws no wick.
		highEdge, lowEdge := bodyTop, bodyBot
		if highPos.Y > lowPos.Y {
			highEdge, lowEdge = bodyBot, bodyTop
		}
		for _, wick := range [2][2]float32{{highPos.Y, highEdge}, {lowPos.Y, lowEdge}} {
			if wick[0] >= bodyTop && wick[0] <= bodyBot {
				continue
			}
			objects = append(objects, &canvas.Line{
				Position1:   geo.NewPos(cx, wick[0]),
				Position2:   geo.NewPos(cx, wick[1]),
				StrokeColor: c,
				StrokeWidth: wickWidth,
			})
		}

		// Animation
		if ctx.Animate && ctx.AnimProgress < 1 {
			midY := (openPos.Y + closePos.Y) / 2
			body.Move(geo.NewPos(cx-bw/2, mathutil.LerpF32(midY, bodyTop, ctx.AnimProgress)))
			body.Resize(geo.NewSize(bw, mathutil.LerpF32(1, bodyH, ctx.AnimProgress)))
		}
	}

	return objects
}
