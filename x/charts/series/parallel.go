package series

import (
	"fmt"
	"image/color"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// ParallelAxis defines one axis in a parallel coordinate chart.
type ParallelAxis struct {
	Name string
	Min  float64 // 0 = auto
	Max  float64 // 0 = auto
}

// ParallelSeries configures a parallel coordinates chart.
// Data is a 2D array: each row is one data line, each column maps to an axis.
type ParallelSeries struct {
	SeriesName string
	Axes       []ParallelAxis
	Data       [][]float64 // Data[line][axis]
	LineWidth  float32
}

func (s *ParallelSeries) SeriesType() Type { return TypeParallel }
func (s *ParallelSeries) Name() string     { return s.SeriesName }
func (s *ParallelSeries) sealed()          {}

func init() {
	RegisterRenderer(TypeParallel, func() Renderer { return &parallelRenderer{} })
}

type parallelRenderer struct{}

func (r *parallelRenderer) Render(ctx RenderContext) []core.CanvasObject {
	ps, ok := ctx.Series.(*ParallelSeries)
	if !ok || len(ps.Axes) == 0 || len(ps.Data) == 0 {
		return nil
	}

	vp := ctx.Viewport
	nAxes := len(ps.Axes)
	lineWidth := ResolveFloat32(ps.LineWidth, DefaultParallelWidth)

	// Compute axis ranges
	type axisRange struct{ min, max float64 }
	ranges := make([]axisRange, nAxes)
	for i, ax := range ps.Axes {
		ranges[i] = axisRange{ax.Min, ax.Max}
		if ranges[i].min == 0 && ranges[i].max == 0 {
			ranges[i].min = 1e18
			ranges[i].max = -1e18
			for _, row := range ps.Data {
				if i < len(row) {
					if row[i] < ranges[i].min {
						ranges[i].min = row[i]
					}
					if row[i] > ranges[i].max {
						ranges[i].max = row[i]
					}
				}
			}
		}
		if ranges[i].max <= ranges[i].min {
			ranges[i].max = ranges[i].min + 1
		}
	}

	margin := float32(20)
	axisGap := (vp.W - 2*margin) / float32(nAxes-1)
	axisTop := vp.Y + margin
	axisBot := vp.Y + vp.H - margin - 16 // room for labels

	palette := DefaultPalette

	var objects []core.CanvasObject

	// Draw axis lines and labels
	axisColor := color.NRGBA{R: 150, G: 150, B: 150, A: 200}
	for i, ax := range ps.Axes {
		x := vp.X + margin + float32(i)*axisGap
		line := &canvas.Line{
			Position1:   geo.NewPos(x, axisTop),
			Position2:   geo.NewPos(x, axisBot),
			StrokeColor: axisColor,
			StrokeWidth: 1,
		}
		objects = append(objects, line)

		label := canvas.NewText(ax.Name, color.NRGBA{R: 180, G: 180, B: 180, A: 255})
		label.TextSize = 10
		label.Alignment = bellui.TextAlignCenter
		labelW := float32(len(ax.Name))*6 + 4
		label.Move(geo.NewPos(x-labelW/2, axisBot+2))
		label.Resize(geo.NewSize(labelW, 12))
		objects = append(objects, label)

		// Min/Max labels
		minLabel := canvas.NewText(fmt.Sprintf("%.0f", ranges[i].min), color.NRGBA{R: 140, G: 140, B: 140, A: 200})
		minLabel.TextSize = 8
		minLabel.Move(geo.NewPos(x+3, axisBot-10))
		minLabel.Resize(geo.NewSize(40, 10))
		maxLabel := canvas.NewText(fmt.Sprintf("%.0f", ranges[i].max), color.NRGBA{R: 140, G: 140, B: 140, A: 200})
		maxLabel.TextSize = 8
		maxLabel.Move(geo.NewPos(x+3, axisTop))
		maxLabel.Resize(geo.NewSize(40, 10))
		objects = append(objects, minLabel, maxLabel)
	}

	// Draw data lines
	for li, row := range ps.Data {
		c := palette[li%len(palette)]
		for ai := 0; ai < nAxes-1 && ai < len(row)-1; ai++ {
			x1 := vp.X + margin + float32(ai)*axisGap
			x2 := vp.X + margin + float32(ai+1)*axisGap
			norm1 := (row[ai] - ranges[ai].min) / (ranges[ai].max - ranges[ai].min)
			norm2 := (row[ai+1] - ranges[ai+1].min) / (ranges[ai+1].max - ranges[ai+1].min)
			y1 := axisBot - float32(norm1)*(axisBot-axisTop)
			y2 := axisBot - float32(norm2)*(axisBot-axisTop)

			seg := &canvas.Line{
				Position1:   geo.NewPos(x1, y1),
				Position2:   geo.NewPos(x2, y2),
				StrokeColor: c,
				StrokeWidth: lineWidth,
			}
			objects = append(objects, seg)
		}
	}

	return objects
}
