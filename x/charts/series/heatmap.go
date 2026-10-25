package series

import (
	"image"
	"image/color"
	"math"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/x/charts/coord"
	"github.com/belluikit/bellui/x/charts/internal/heatgrid"
)

// HeatmapSeries configures a heatmap chart series.
// Data is a 2D grid stored row-major: Data[row*Cols + col].
type HeatmapSeries struct {
	SeriesName string
	Data       []float64 // row-major values
	Rows       int
	Cols       int

	// XLabels and YLabels for axis ticks (optional).
	XLabels []string
	YLabels []string

	// Min/Max for color mapping. If both zero, auto-computed from data.
	Min, Max float64

	// ColorStops defines the gradient. If nil, a default blue-red gradient is used.
	ColorStops []HeatmapColorStop
}

// HeatmapColorStop maps a normalized [0, 1] value to a color.
type HeatmapColorStop struct {
	Position float64 // 0.0 to 1.0
	Color    color.Color
}

// SeriesType returns TypeHeatmap.
func (s *HeatmapSeries) SeriesType() Type { return TypeHeatmap }

// Name returns the series display name.
func (s *HeatmapSeries) Name() string { return s.SeriesName }

func (s *HeatmapSeries) sealed() {}

// Value returns the data value at (row, col).
func (s *HeatmapSeries) Value(row, col int) float64 {
	idx := row*s.Cols + col
	if idx >= 0 && idx < len(s.Data) {
		return s.Data[idx]
	}
	return 0
}

func init() {
	RegisterRenderer(TypeHeatmap, func() Renderer { return &heatmapRenderer{} })
}

type heatmapRenderer struct{}

func (r *heatmapRenderer) Render(ctx RenderContext) []core.CanvasObject {
	hs, ok := ctx.Series.(*HeatmapSeries)
	if !ok || len(hs.Data) == 0 || hs.Rows <= 0 || hs.Cols <= 0 {
		return nil
	}

	vp := ctx.Viewport

	// Compute data range for color mapping
	dMin, dMax := hs.Min, hs.Max
	if dMin == 0 && dMax == 0 {
		dMin = math.Inf(1)
		dMax = math.Inf(-1)
		for _, v := range hs.Data {
			if v < dMin {
				dMin = v
			}
			if v > dMax {
				dMax = v
			}
		}
	}

	stops := hs.ColorStops
	if len(stops) == 0 {
		stops = defaultHeatmapStops()
	}

	// Render as a single Raster image for efficiency
	raster := canvas.NewRaster(func(w, h int) image.Image {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		cellW := float64(w) / float64(hs.Cols)
		cellH := float64(h) / float64(hs.Rows)

		for row := 0; row < hs.Rows; row++ {
			for col := 0; col < hs.Cols; col++ {
				v := hs.Value(row, col)
				norm := 0.0
				if dMax > dMin {
					norm = (v - dMin) / (dMax - dMin)
				}
				c := interpolateColor(stops, norm)

				x0 := int(float64(col) * cellW)
				y0 := int(float64(row) * cellH)
				x1 := int(float64(col+1) * cellW)
				y1 := int(float64(row+1) * cellH)

				for py := y0; py < y1; py++ {
					for px := x0; px < x1; px++ {
						img.Set(px, py, c)
					}
				}
			}
		}
		return img
	})

	// The cells leave room for the row and column labels, which were only
	// ever shown in the tooltip (#343). The chart hit-tests the same cells.
	cells := heatgrid.Layout(vp, hs.XLabels, hs.YLabels)
	raster.Move(geo.NewPos(cells.X, cells.Y))
	raster.Resize(geo.NewSize(cells.W, cells.H))
	raster.ScaleMode = canvas.ImageScalePixels

	objects := []core.CanvasObject{raster}
	return append(objects, heatmapLabels(ctx, hs, vp, cells)...)
}

// heatmapLabels draws the row labels right-aligned left of the cells and the
// column labels centred below them, every one that fits: a column label
// wider than its cell takes every n-th column so the labels do not overlap.
func heatmapLabels(ctx RenderContext, hs *HeatmapSeries, vp, cells coord.Rect) []core.CanvasObject {
	c := backgroundLabelColor(ctx, color.NRGBA{R: 180, G: 180, B: 180, A: 255})
	var objects []core.CanvasObject
	label := func(text string, x, y, w float32, align bellui.TextAlign) {
		t := canvas.NewText(text, c)
		t.TextSize = heatgrid.LabelSize
		t.Alignment = align
		t.Move(geo.NewPos(x, y))
		t.Resize(geo.NewSize(w, heatgrid.LabelSize+2))
		objects = append(objects, t)
	}

	cellH := cells.H / float32(hs.Rows)
	for row := 0; row < hs.Rows && row < len(hs.YLabels); row++ {
		y := cells.Y + (float32(row)+0.5)*cellH - (heatgrid.LabelSize+2)/2
		label(hs.YLabels[row], vp.X, y, cells.X-vp.X-heatgrid.Gap(), bellui.TextAlignTrailing)
	}

	cellW := cells.W / float32(hs.Cols)
	if cellW <= 0 {
		return objects
	}
	var widest float32
	for _, l := range hs.XLabels {
		if w := heatgrid.LabelWidth(l); w > widest {
			widest = w
		}
	}
	step := int(math.Ceil(float64((widest + heatgrid.Gap()) / cellW)))
	if step < 1 {
		step = 1
	}
	y := cells.Y + cells.H + heatgrid.Gap()
	for col := 0; col < hs.Cols && col < len(hs.XLabels); col += step {
		x := cells.X + (float32(col)+0.5)*cellW - widest/2
		label(hs.XLabels[col], x, y, widest, bellui.TextAlignCenter)
	}
	return objects
}

func defaultHeatmapStops() []HeatmapColorStop {
	return []HeatmapColorStop{
		{Position: 0.0, Color: color.NRGBA{R: 49, G: 54, B: 149, A: 255}},   // deep blue
		{Position: 0.25, Color: color.NRGBA{R: 69, G: 117, B: 180, A: 255}}, // blue
		{Position: 0.5, Color: color.NRGBA{R: 255, G: 255, B: 191, A: 255}}, // yellow
		{Position: 0.75, Color: color.NRGBA{R: 244, G: 109, B: 67, A: 255}}, // orange
		{Position: 1.0, Color: color.NRGBA{R: 165, G: 0, B: 38, A: 255}},    // deep red
	}
}

func interpolateColor(stops []HeatmapColorStop, norm float64) color.Color {
	if norm <= 0 {
		return stops[0].Color
	}
	if norm >= 1 {
		return stops[len(stops)-1].Color
	}
	for i := 1; i < len(stops); i++ {
		if norm <= stops[i].Position {
			t := (norm - stops[i-1].Position) / (stops[i].Position - stops[i-1].Position)
			return lerpColor(stops[i-1].Color, stops[i].Color, t)
		}
	}
	return stops[len(stops)-1].Color
}

func lerpColor(a, b color.Color, t float64) color.Color {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return color.NRGBA{
		R: uint8(float64(ar>>8) + t*float64(int(br>>8)-int(ar>>8))),
		G: uint8(float64(ag>>8) + t*float64(int(bg>>8)-int(ag>>8))),
		B: uint8(float64(ab>>8) + t*float64(int(bb>>8)-int(ab>>8))),
		A: uint8(float64(aa>>8) + t*float64(int(ba>>8)-int(aa>>8))),
	}
}
