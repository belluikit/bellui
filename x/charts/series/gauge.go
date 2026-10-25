package series

import (
	"fmt"
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// GaugeSeries configures a gauge/meter chart series.
type GaugeSeries struct {
	SeriesName string
	Value      float64 // current value
	Min        float64 // minimum (default 0)
	Max        float64 // maximum (default 100)

	// StartAngle and EndAngle in degrees, with 0 at the top and angles growing
	// clockwise. Min sits at StartAngle and values sweep counter-clockwise, by
	// decreasing angle, to Max at EndAngle.
	// Defaults: StartAngle=135, EndAngle=-135, a 270-degree arc from the
	// bottom right over the top to the bottom left, with the gap at the bottom.
	StartAngle float32
	EndAngle   float32

	// Sectors define colored ranges on the gauge (e.g., green/yellow/red zones).
	Sectors []GaugeSector

	// ShowValue displays the current value as text in the center.
	ShowValue bool

	// Radius as fraction of available space (0-1). Default 0.85.
	Radius float32
}

// GaugeSector defines a colored range on the gauge arc.
type GaugeSector struct {
	Min, Max float64
	Color    color.Color
}

// SeriesType returns TypeGauge.
func (s *GaugeSeries) SeriesType() Type { return TypeGauge }

// Name returns the series display name.
func (s *GaugeSeries) Name() string { return s.SeriesName }

func (s *GaugeSeries) sealed() {}

// valueRingOuter and valueRingInner bound the thin value ring drawn inside
// the coloured ranges, as fractions of the gauge radius; the ranges and the
// track take 0.7 to 1.
const (
	valueRingOuter float32 = 0.66
	valueRingInner float32 = 0.58
)

func init() {
	RegisterRenderer(TypeGauge, func() Renderer { return &gaugeRenderer{} })
}

type gaugeRenderer struct{}

func (r *gaugeRenderer) Render(ctx RenderContext) []core.CanvasObject {
	gs, ok := ctx.Series.(*GaugeSeries)
	if !ok {
		return nil
	}

	vp := ctx.Viewport
	centerX := vp.X + vp.W/2
	centerY := vp.Y + vp.H*0.55 // slightly below center for the arc

	maxDim := vp.W / 2
	if vp.H/2 < maxDim {
		maxDim = vp.H / 2
	}

	radius := ResolveFloat32(gs.Radius, DefaultGaugeRadius)
	outerR := maxDim * radius

	gMin := gs.Min
	gMax := gs.Max
	if gMax <= gMin {
		gMax = 100
	}

	startAngle := gs.StartAngle
	endAngle := gs.EndAngle
	if startAngle == 0 && endAngle == 0 {
		// The gap at the bottom. The old 225/-45 left it on the left (#343).
		startAngle = 135
		endAngle = -135
	}

	totalSweep := startAngle - endAngle
	if totalSweep < 0 {
		totalSweep += 360
	}

	var objects []core.CanvasObject
	arcDiameter := outerR * 2
	arcSize := geo.NewSize(arcDiameter, arcDiameter)
	arcPos := geo.NewPos(centerX-outerR, centerY-outerR)

	// Background track arc
	trackColor := color.NRGBA{R: 60, G: 60, B: 60, A: 180}
	trackArc := canvas.NewArc(endAngle, startAngle, 0.7, trackColor)
	trackArc.Move(arcPos)
	trackArc.Resize(arcSize)
	objects = append(objects, trackArc)

	// Sector arcs (colored zones)
	for _, sector := range gs.Sectors {
		sMin := (sector.Min - gMin) / (gMax - gMin)
		sMax := (sector.Max - gMin) / (gMax - gMin)
		if sMin < 0 {
			sMin = 0
		}
		if sMax > 1 {
			sMax = 1
		}
		// Map normalized range to angle range
		// startAngle is the beginning (min), sweeping counter-clockwise to endAngle (max)
		sStartAngle := startAngle - float32(sMin)*totalSweep
		sEndAngle := startAngle - float32(sMax)*totalSweep

		sectorArc := canvas.NewArc(sEndAngle, sStartAngle, 0.7, sector.Color)
		sectorArc.Move(arcPos)
		sectorArc.Resize(arcSize)
		objects = append(objects, sectorArc)
	}

	// Value arc (from min to current value)
	valNorm := (gs.Value - gMin) / (gMax - gMin)
	if valNorm < 0 {
		valNorm = 0
	}
	if valNorm > 1 {
		valNorm = 1
	}
	valAngle := startAngle - float32(valNorm)*totalSweep

	valueColor := ctx.Color
	if valueColor == nil {
		valueColor = color.NRGBA{R: 91, G: 143, B: 249, A: 255}
	}

	if valNorm > 0.001 {
		valueArc := canvas.NewArc(valAngle, startAngle, 0.7, valueColor)
		valueArc.Move(arcPos)
		valueArc.Resize(arcSize)
		if len(gs.Sectors) > 0 {
			// With coloured ranges the value runs as a thin ring inside them,
			// so it no longer paints over the ranges below the value (#343).
			innerR := outerR * valueRingOuter
			valueArc.CutoutRatio = valueRingInner / valueRingOuter
			valueArc.Move(geo.NewPos(centerX-innerR, centerY-innerR))
			valueArc.Resize(geo.NewSize(innerR*2, innerR*2))
		}
		objects = append(objects, valueArc)
	}

	// Needle line from center to the value position
	needleAngle := float64(valAngle-90) * math.Pi / 180
	needleLen := outerR * 0.85
	needleX := centerX + float32(math.Cos(needleAngle))*needleLen
	needleY := centerY + float32(math.Sin(needleAngle))*needleLen

	needle := &canvas.Line{
		Position1:   geo.NewPos(centerX, centerY),
		Position2:   geo.NewPos(needleX, needleY),
		StrokeColor: valueColor,
		StrokeWidth: 2.5,
	}
	objects = append(objects, needle)

	// Center dot
	dotR := float32(5)
	dot := canvas.NewCircle(valueColor)
	dot.Position1 = geo.NewPos(centerX-dotR, centerY-dotR)
	dot.Position2 = geo.NewPos(centerX+dotR, centerY+dotR)
	objects = append(objects, dot)

	// Value text
	if gs.ShowValue {
		valText := fmt.Sprintf("%.1f", gs.Value)
		label := canvas.NewText(valText, backgroundLabelColor(ctx, color.NRGBA{R: 230, G: 230, B: 230, A: 255}))
		label.TextSize = 20
		label.TextStyle = bellui.TextStyle{Bold: true}
		label.Alignment = bellui.TextAlignCenter
		labelW := float32(len(valText))*12 + 10
		label.Move(geo.NewPos(centerX-labelW/2, centerY+outerR*0.3))
		label.Resize(geo.NewSize(labelW, 24))
		objects = append(objects, label)

		// Name below value
		if gs.SeriesName != "" {
			nameLabel := canvas.NewText(gs.SeriesName, color.NRGBA{R: 150, G: 150, B: 150, A: 255})
			nameLabel.TextSize = 12
			nameLabel.Alignment = bellui.TextAlignCenter
			nameW := float32(len(gs.SeriesName))*7 + 10
			nameLabel.Move(geo.NewPos(centerX-nameW/2, centerY+outerR*0.3+26))
			nameLabel.Resize(geo.NewSize(nameW, 14))
			objects = append(objects, nameLabel)
		}
	}

	return objects
}
