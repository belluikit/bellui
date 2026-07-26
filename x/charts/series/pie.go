package series

import (
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"

	"github.com/belluikit/bellui/x/charts/coord"
)

// PieDataItem represents a single slice in a pie chart.
type PieDataItem struct {
	Name  string
	Value float64
	Color color.Color // optional per-slice color override
}

// PieSeries configures a pie/doughnut chart series.
type PieSeries struct {
	SeriesName string
	Data       []PieDataItem

	// Radius as fraction of available space [0, 1]. Default 0.75.
	Radius float32
	// InnerRadius as fraction of Radius [0, 1). 0 = pie, >0 = doughnut. Default 0.
	InnerRadius float32

	// StartAngle in degrees (0 = top). Default 0.
	StartAngle float32

	// ShowLabels displays slice names outside the pie.
	ShowLabels bool

	// PadAngle adds a gap (in degrees) between slices.
	PadAngle float32
}

// SeriesType returns TypePie.
func (s *PieSeries) SeriesType() Type { return TypePie }

// Name returns the series display name.
func (s *PieSeries) Name() string { return s.SeriesName }

func (s *PieSeries) sealed() {}

// PieLayout holds the computed geometry for a pie chart, shared between
// rendering and hit-test registration so the calculation is done once.
type PieLayout struct {
	CenterX, CenterY float32
	OuterR           float32
	Total            float64
	StartAngle       float32
}

// ComputePieLayout calculates the center, radius, and total for a PieSeries
// within the given viewport. Returns nil if there is no data to render.
func (s *PieSeries) ComputePieLayout(vp coord.Rect) *PieLayout {
	if len(s.Data) == 0 {
		return nil
	}

	labelSpace := float32(50)
	if !s.ShowLabels {
		labelSpace = 10
	}
	availW := vp.W - labelSpace*2
	availH := vp.H - labelSpace*2
	if availW < 0 {
		availW = vp.W
	}
	if availH < 0 {
		availH = vp.H
	}
	centerX := vp.X + vp.W/2
	centerY := vp.Y + labelSpace + availH/2

	maxDim := availW / 2
	if availH/2 < maxDim {
		maxDim = availH / 2
	}
	radius := ResolveFloat32(s.Radius, DefaultPieRadius)
	outerR := maxDim * radius

	total := 0.0
	for _, d := range s.Data {
		total += math.Abs(d.Value)
	}
	if total == 0 {
		return nil
	}

	return &PieLayout{
		CenterX:    centerX,
		CenterY:    centerY,
		OuterR:     outerR,
		Total:      total,
		StartAngle: s.StartAngle,
	}
}

func init() {
	RegisterRenderer(TypePie, func() Renderer { return &pieRenderer{} })
}

type pieRenderer struct{}

func (r *pieRenderer) Render(ctx RenderContext) []core.CanvasObject {
	ps, ok := ctx.Series.(*PieSeries)
	if !ok {
		return nil
	}

	layout := ps.ComputePieLayout(ctx.Viewport)
	if layout == nil {
		return nil
	}

	centerX := layout.CenterX
	centerY := layout.CenterY
	outerR := layout.OuterR
	total := layout.Total
	startAngle := layout.StartAngle

	cutoutRatio := ps.InnerRadius
	if cutoutRatio < 0 {
		cutoutRatio = 0
	}
	padAngle := ps.PadAngle

	arcDiameter := outerR * 2
	arcSize := geo.NewSize(arcDiameter, arcDiameter)

	objects := make([]core.CanvasObject, 0, len(ps.Data)*2)

	for i, d := range ps.Data {
		fraction := math.Abs(d.Value) / total
		sweepAngle := float32(fraction * 360)

		if sweepAngle <= 0 {
			startAngle += sweepAngle + padAngle
			continue
		}

		// Determine color
		sliceColor := d.Color
		if sliceColor == nil {
			sliceColor = r.paletteColor(ctx, i)
		}

		actualStart := startAngle + padAngle/2
		actualEnd := startAngle + sweepAngle - padAngle/2

		// Animation: sweep from 0
		if ctx.Animate && ctx.AnimProgress < 1 {
			actualEnd = actualStart + (actualEnd-actualStart)*ctx.AnimProgress
		}

		arc := canvas.NewArc(actualStart, actualEnd, cutoutRatio, sliceColor)
		// Position the arc's bounding box so its visual center aligns with (centerX, centerY)
		arc.Move(geo.NewPos(centerX-outerR, centerY-outerR))
		arc.Resize(arcSize)
		objects = append(objects, arc)

		// Labels
		if ps.ShowLabels && d.Name != "" {
			midAngle := (actualStart + actualEnd) / 2
			labelR := outerR + 12
			// Convert angle to radians (0=top, clockwise)
			rad := float64(midAngle-90) * math.Pi / 180
			lx := centerX + float32(math.Cos(rad))*labelR
			ly := centerY + float32(math.Sin(rad))*labelR

			label := canvas.NewText(d.Name, sliceColor)
			label.TextSize = 11
			labelW := float32(len(d.Name))*7 + 10 // approximate width with padding
			// Determine side by checking if the label point is left or right of center
			if lx < centerX {
				// Left side: text ends at the computed point
				label.Alignment = bellui.TextAlignTrailing
				label.Move(geo.NewPos(lx-labelW, ly-7))
			} else {
				// Right side: text starts at the computed point
				label.Alignment = bellui.TextAlignLeading
				label.Move(geo.NewPos(lx, ly-7))
			}
			label.Resize(geo.NewSize(labelW, 14))
			objects = append(objects, label)
		}

		startAngle += sweepAngle
	}

	return objects
}

func (r *pieRenderer) paletteColor(ctx RenderContext, sliceIndex int) color.Color {
	return DefaultPalette[sliceIndex%len(DefaultPalette)]
}
