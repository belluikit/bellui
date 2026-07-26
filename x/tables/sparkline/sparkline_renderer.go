package sparkline

import (
	"image"
	"image/color"
	"math"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

type sparklineRenderer struct {
	sparkline *Sparkline
	raster    *canvas.Raster
	objects   []core.CanvasObject
	cache     *image.NRGBA
}

func newSparklineRenderer(s *Sparkline) *sparklineRenderer {
	r := &sparklineRenderer{sparkline: s}
	r.raster = canvas.NewRaster(r.generate)
	r.objects = []core.CanvasObject{r.raster}
	return r
}

func (r *sparklineRenderer) Layout(size geo.Size) {
	r.raster.Resize(size)
}

func (r *sparklineRenderer) MinSize() geo.Size {
	return geo.NewSize(50, 20)
}

func (r *sparklineRenderer) Refresh() {
	r.sparkline.dirty = true
	r.cache = nil
	r.raster.Refresh()
}

func (r *sparklineRenderer) Objects() []core.CanvasObject {
	return r.objects
}

func (r *sparklineRenderer) Destroy() {}

func (r *sparklineRenderer) generate(w, h int) image.Image {
	// Return cached image if nothing changed
	if r.cache != nil && !r.sparkline.dirty &&
		r.sparkline.lastW == w && r.sparkline.lastH == h {
		return r.cache
	}

	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	data := r.sparkline.Data
	if len(data) < 1 || w < 2 || h < 2 {
		r.cache = img
		r.sparkline.dirty = false
		r.sparkline.lastW = w
		r.sparkline.lastH = h
		return img
	}

	opts := r.sparkline.Opts
	pad := int(opts.Padding)
	drawW := w - pad*2
	drawH := h - pad*2
	if drawW < 2 || drawH < 2 {
		r.cache = img
		return img
	}

	// Compute domain (min/max)
	minV, maxV := data[0], data[0]
	for _, v := range data {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	if opts.DomainMin != nil {
		minV = *opts.DomainMin
	}
	if opts.DomainMax != nil {
		maxV = *opts.DomainMax
	}
	rang := maxV - minV
	if rang == 0 {
		rang = 1
	}

	// Draw axis line at y=0 if enabled and 0 is within the domain
	if opts.ShowAxis && minV <= 0 && maxV >= 0 {
		axisY := pad + drawH - int(((0-minV)/rang)*float64(drawH))
		axisColor := toNRGBA(opts.AxisColor)
		for x := pad; x < pad+drawW; x++ {
			if axisY >= 0 && axisY < h {
				img.SetNRGBA(x, axisY, axisColor)
			}
		}
	}

	switch r.sparkline.ChartType {
	case Line, Area:
		r.drawLine(img, data, minV, rang, pad, drawW, drawH)
	case Bar, Column:
		r.drawColumn(img, data, minV, maxV, rang, pad, drawW, drawH)
	}

	// Draw highlight point
	if opts.HighlightIndex >= 0 && opts.HighlightIndex < len(data) && len(data) >= 2 {
		i := opts.HighlightIndex
		x := pad + int(float64(i)*float64(drawW)/float64(len(data)-1))
		y := pad + drawH - int(((data[i]-minV)/rang)*float64(drawH))
		hlColor := toNRGBA(opts.HighlightColor)
		hlR := int(opts.HighlightSize)
		if hlR < 2 {
			hlR = 2
		}
		drawCircle(img, x, y, hlR, hlColor)
	}

	r.cache = img
	r.sparkline.dirty = false
	r.sparkline.lastW = w
	r.sparkline.lastH = h
	return img
}

func (r *sparklineRenderer) drawLine(img *image.NRGBA, data []float64, minV, rang float64, pad, drawW, drawH int) {
	opts := r.sparkline.Opts
	strokeColor := toNRGBA(opts.Stroke)
	fillColor := toNRGBA(opts.Fill)
	n := len(data)
	if n < 2 {
		return
	}

	// Convert data to pixel coordinates
	points := make([][2]int, n)
	for i, v := range data {
		x := pad + int(float64(i)*float64(drawW)/float64(n-1))
		y := pad + drawH - int(((v-minV)/rang)*float64(drawH))
		points[i] = [2]int{x, y}
	}

	// Draw fill area (if Area type)
	if r.sparkline.ChartType == Area && fillColor.A > 0 {
		baseY := pad + drawH
		for i := 0; i < n-1; i++ {
			x1, y1 := points[i][0], points[i][1]
			x2, y2 := points[i+1][0], points[i+1][1]
			for x := x1; x <= x2; x++ {
				if x < 0 || x >= img.Bounds().Max.X {
					continue
				}
				t := float64(x-x1) / float64(x2-x1+1)
				y := int(float64(y1) + t*float64(y2-y1))
				for dy := y; dy <= baseY; dy++ {
					if dy >= 0 && dy < img.Bounds().Max.Y {
						img.SetNRGBA(x, dy, fillColor)
					}
				}
			}
		}
	}

	// Draw line segments
	for i := 0; i < n-1; i++ {
		lineColor := strokeColor
		if opts.ItemStyler != nil {
			if c := opts.ItemStyler(i, data[i]); c != nil {
				lineColor = toNRGBA(c)
			}
		}
		drawLineSegment(img, points[i][0], points[i][1], points[i+1][0], points[i+1][1], lineColor)
	}

	// Draw markers
	if opts.ShowMarkers && opts.MarkerSize > 0 {
		mr := int(opts.MarkerSize)
		markerColor := toNRGBA(opts.MarkerFill)
		for i, p := range points {
			mc := markerColor
			if opts.ItemStyler != nil {
				if c := opts.ItemStyler(i, data[i]); c != nil {
					mc = toNRGBA(c)
				}
			}
			drawCircle(img, p[0], p[1], mr, mc)
		}
	}
}

func (r *sparklineRenderer) drawColumn(img *image.NRGBA, data []float64, minV, maxV, rang float64, pad, drawW, drawH int) {
	opts := r.sparkline.Opts
	posColor := toNRGBA(opts.Stroke)
	negColor := toNRGBA(opts.NegativeColor)
	n := len(data)
	if n == 0 {
		return
	}

	barW := drawW / n
	if barW < 1 {
		barW = 1
	}
	gap := 1
	if barW <= 2 {
		gap = 0
	}

	// Zero line position
	zeroY := pad + drawH
	if minV < 0 && maxV > 0 {
		zeroY = pad + drawH - int(((0-minV)/rang)*float64(drawH))
	}

	for i, v := range data {
		x := pad + i*barW
		valY := pad + drawH - int(((v-minV)/rang)*float64(drawH))

		fillColor := posColor
		if v < 0 {
			fillColor = negColor
		}
		if opts.ItemStyler != nil {
			if c := opts.ItemStyler(i, v); c != nil {
				fillColor = toNRGBA(c)
			}
		}

		// Draw from zeroY to valY
		top, bottom := valY, zeroY
		if top > bottom {
			top, bottom = bottom, top
		}
		for bx := x + gap; bx < x+barW-gap && bx < img.Bounds().Max.X; bx++ {
			for by := top; by < bottom && by < img.Bounds().Max.Y; by++ {
				if bx >= 0 && by >= 0 {
					img.SetNRGBA(bx, by, fillColor)
				}
			}
		}
	}
}

// Bresenham line drawing
func drawLineSegment(img *image.NRGBA, x0, y0, x1, y1 int, c color.NRGBA) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx, sy := 1, 1
	if x0 >= x1 {
		sx = -1
	}
	if y0 >= y1 {
		sy = -1
	}
	err := dx + dy

	bounds := img.Bounds()
	for {
		if x0 >= 0 && x0 < bounds.Max.X && y0 >= 0 && y0 < bounds.Max.Y {
			img.SetNRGBA(x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func drawCircle(img *image.NRGBA, cx, cy, r int, c color.NRGBA) {
	bounds := img.Bounds()
	for dx := -r; dx <= r; dx++ {
		for dy := -r; dy <= r; dy++ {
			if dx*dx+dy*dy <= r*r {
				px, py := cx+dx, cy+dy
				if px >= 0 && px < bounds.Max.X && py >= 0 && py < bounds.Max.Y {
					img.SetNRGBA(px, py, c)
				}
			}
		}
	}
}

func abs(x int) int {
	return int(math.Abs(float64(x)))
}

func toNRGBA(c color.Color) color.NRGBA {
	if c == nil {
		return color.NRGBA{}
	}
	if n, ok := c.(color.NRGBA); ok {
		return n
	}
	r, g, b, a := c.RGBA()
	if a == 0 {
		return color.NRGBA{}
	}
	return color.NRGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: uint8(a >> 8),
	}
}
