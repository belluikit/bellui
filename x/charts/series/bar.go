package series

import (
	"image/color"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/internal/mathutil"
)

// BarSeries configures a bar chart series.
type BarSeries struct {
	SeriesName string
	Data       []float64

	XAxisIndex int
	YAxisIndex int

	// BarWidth as a fraction of category spacing (0-1). 0 = auto.
	BarWidth float32
	// BarGap as a fraction of bar width between bars in same category. 0 = no gap.
	BarGap float32
	// Stack group name. Bars with the same Stack value are stacked.
	Stack string

	// BarStyle
	Color        color.Color
	BorderColor  color.Color
	BorderWidth  float32
	CornerRadius float32
}

// SeriesType returns TypeBar.
func (s *BarSeries) SeriesType() Type { return TypeBar }

// Name returns the series display name.
func (s *BarSeries) Name() string { return s.SeriesName }

func (s *BarSeries) sealed() {}

// GetYAxisIndex returns the Y axis index for this series.
func (s *BarSeries) GetYAxisIndex() int { return s.YAxisIndex }

// StackGroup returns the stack group name.
func (s *BarSeries) StackGroup() string { return s.Stack }

// StackData returns the data used for stack offset calculations.
func (s *BarSeries) StackData() []float64 { return s.Data }

// RegisterHitAreas registers rectangle-based hit areas for the bar series.
func (s *BarSeries) RegisterHitAreas(collector HitAreaCollector, seriesIndex int, coordSys coord.System, plotRect coord.Rect) {
	n := len(s.Data)
	if n == 0 {
		return
	}
	barWidth := ResolveFloat32(s.BarWidth, DefaultBarWidth)
	categoryWidth := categoryWidthOf(coordSys, n)
	bw := categoryWidth * barWidth

	baselinePos := coordSys.DataToPixel(0, 0)
	baseY := baselinePos.Y

	bars := make([]HitBar, n)
	for i, v := range s.Data {
		pos := coordSys.DataToPixel(float64(i), v)
		bx := pos.X - bw/2
		by := pos.Y
		bh := baseY - pos.Y
		if bh < 0 {
			by = baseY
			bh = -bh
		}
		bars[i] = HitBar{
			DataIndex: i,
			Rect:      [4]float32{bx, by, bw, bh},
			PixelPos:  pos,
			Value:     HitValue{X: float64(i), Y: v},
		}
	}
	collector.AddHitBars(seriesIndex, s.SeriesName, TypeBar, bars)
}

// AppendValues appends data points to the bar series.
func (s *BarSeries) AppendValues(values ...float64) {
	s.Data = append(s.Data, values...)
}

// DataRange returns the data extent for axis scaling. Includes 0 in Y range.
func (s *BarSeries) DataRange() (xMin, xMax, yMin, yMax float64) {
	if len(s.Data) == 0 {
		return 0, 0, 0, 0
	}
	xMin, xMax = 0, float64(len(s.Data)-1)
	yMin, yMax = 0, 0 // bars always include baseline
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
	RegisterRenderer(TypeBar, func() Renderer { return &barRenderer{} })
}

type barRenderer struct{}

func (r *barRenderer) Render(ctx RenderContext) []core.CanvasObject {
	bs, ok := ctx.Series.(*BarSeries)
	if !ok || len(bs.Data) == 0 {
		return nil
	}

	n := len(bs.Data)

	// Compute bar geometry
	barWidth := ResolveFloat32(bs.BarWidth, DefaultBarWidth)

	categoryWidth := categoryWidthOf(ctx.Coord, n)
	bw := categoryWidth * barWidth

	fillColor := ResolveColor(bs.Color, ctx.Color)

	// Baseline pixel Y (data Y = 0)
	baselinePos := ctx.Coord.DataToPixel(0, 0)
	baseY := baselinePos.Y

	// Stack offsets: if this series has a Stack group, read the cumulative offset
	var stackBase []float64
	if bs.Stack != "" && ctx.StackOffsets != nil {
		stackBase = ctx.StackOffsets[bs.Stack]
	}

	objects := make([]core.CanvasObject, 0, n)
	for i, v := range bs.Data {
		// Apply stack offset
		offset := 0.0
		if stackBase != nil && i < len(stackBase) {
			offset = stackBase[i]
		}
		pos := ctx.Coord.DataToPixel(float64(i), v+offset)
		basePos := ctx.Coord.DataToPixel(float64(i), offset)

		barX := pos.X - bw/2
		barY := pos.Y
		barH := basePos.Y - pos.Y

		// Handle negative values
		if barH < 0 {
			barY = basePos.Y
			barH = -barH
		}

		// Animation: scale height from 0, growing from the baseline towards
		// the value: upwards unless the value's pixel is below its base, as
		// it is for a negative value or on an Inverse Y axis.
		if ctx.Animate && ctx.AnimProgress < 1 {
			barH = mathutil.LerpF32(0, barH, ctx.AnimProgress)
			barY = baseY - barH
			if pos.Y > basePos.Y {
				barY = baseY
			}
		}

		var rect *canvas.Rectangle
		if ctx.Pool != nil {
			rect = ctx.Pool.Rect(geo.NewPos(barX, barY), geo.NewSize(bw, barH), fillColor)
		} else {
			rect = canvas.NewRectangle(fillColor)
			rect.Move(geo.NewPos(barX, barY))
			rect.Resize(geo.NewSize(bw, barH))
		}
		rect.CornerRadius = bs.CornerRadius
		if bs.BorderColor != nil {
			rect.StrokeColor = bs.BorderColor
			rect.StrokeWidth = bs.BorderWidth
		}
		objects = append(objects, rect)
	}

	return objects
}
