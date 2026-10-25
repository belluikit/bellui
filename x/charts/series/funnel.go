package series

import (
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// FunnelData holds one stage of a funnel chart.
type FunnelData struct {
	Name  string
	Value float64
	Color color.Color
}

// FunnelSeries configures a funnel chart series.
type FunnelSeries struct {
	SeriesName string
	Data       []FunnelData
	// Sort: "descending" (default, widest on top), "ascending", or "none".
	Sort string
	// DynamicHeight: when true, each stage's height is proportional to its value,
	// creating a V-shape funnel. When false (default), all stages have equal height.
	DynamicHeight bool
}

func (s *FunnelSeries) SeriesType() Type { return TypeFunnel }
func (s *FunnelSeries) Name() string     { return s.SeriesName }
func (s *FunnelSeries) sealed()          {}

func init() {
	RegisterRenderer(TypeFunnel, func() Renderer { return &funnelRenderer{} })
}

type funnelRenderer struct{}

func (r *funnelRenderer) Render(ctx RenderContext) []core.CanvasObject {
	fs, ok := ctx.Series.(*FunnelSeries)
	if !ok || len(fs.Data) == 0 {
		return nil
	}

	vp := ctx.Viewport
	n := len(fs.Data)

	// Find max value for width scaling
	maxVal := 0.0
	for _, d := range fs.Data {
		if d.Value > maxVal {
			maxVal = d.Value
		}
	}
	if maxVal == 0 {
		maxVal = 1
	}

	palette := DefaultPalette

	gap := float32(2)
	centerX := vp.X + vp.W/2

	// Compute stage heights
	totalVal := 0.0
	for _, d := range fs.Data {
		totalVal += d.Value
	}

	var objects []core.CanvasObject
	currentY := vp.Y

	for i, d := range fs.Data {
		frac := float32(d.Value / maxVal)
		w := vp.W * frac * 0.9

		var stageH float32
		if fs.DynamicHeight && totalVal > 0 {
			// Proportional height: stage height based on value
			stageH = vp.H * float32(d.Value/totalVal)
		} else {
			stageH = vp.H / float32(n)
		}

		c := palette[i%len(palette)]
		if d.Color != nil {
			c = d.Color
		}

		rect := canvas.NewRectangle(c)
		rect.CornerRadius = 3
		rect.Move(geo.NewPos(centerX-w/2, currentY+gap/2))
		rect.Resize(geo.NewSize(w, stageH-gap))
		objects = append(objects, rect)

		// Label
		if d.Name != "" {
			label := canvas.NewText(d.Name, labelColorOn(c))
			label.TextSize = 11
			label.Alignment = bellui.TextAlignCenter
			label.Move(geo.NewPos(centerX-w/2, currentY+stageH/2-7))
			label.Resize(geo.NewSize(w, 14))
			objects = append(objects, label)
		}

		currentY += stageH
	}

	return objects
}
