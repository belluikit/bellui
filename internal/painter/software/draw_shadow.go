package software

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/draw"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/painter"
	"github.com/belluikit/bellui/internal/scale"
)

// drawBoxShadow draws a canvas.BoxShadow as the GPU painters do: the shadow's
// rounded rect, padded by twice the blur radius on each side, blurred by a
// Gaussian of sigma painter.BlurSigma(BlurRadius), in the shadow colour. The
// GPU painters approximate that Gaussian with a 9-tap kernel; this computes it
// exactly. A shadow with no blur draws nothing, as there.
//
// Without it a headless render dropped every shadow, so an elevated Card whose
// surface matched the page had no visible edge at all (#876).
func drawBoxShadow(c core.Canvas, shadow *canvas.BoxShadow, pos geo.Position, base *image.NRGBA, clip image.Rectangle) {
	if shadow.BlurRadius <= 0 || shadow.Color == nil {
		return
	}
	spread := shadow.BlurRadius * 2 // as the GPU painters and PaintExtent pad it
	size := shadow.Size()
	x := scale.ToScreenCoordinate(c, pos.X+shadow.OffsetX-spread)
	y := scale.ToScreenCoordinate(c, pos.Y+shadow.OffsetY-spread)
	w := scale.ToScreenCoordinate(c, size.Width+spread*2)
	h := scale.ToScreenCoordinate(c, size.Height+spread*2)
	out := clip.Intersect(image.Rect(x, y, x+w, y+h))
	if w <= 0 || h <= 0 || out.Empty() {
		return
	}

	// The shape's coverage, antialiased as any rectangle is.
	shape := image.NewNRGBA(image.Rect(0, 0, w, h))
	rect := &canvas.Rectangle{FillColor: color.White, CornerRadius: shadow.CornerRadius}
	rect.Resize(size)
	drawRectangle(c, rect, geo.NewPos(spread, spread), shape, shape.Bounds())
	cover := make([]float32, w*h)
	for i := range cover {
		cover[i] = float32(shape.Pix[i*4+3]) / 0xff
	}

	sigma := float64(painter.BlurSigma(shadow.BlurRadius) * c.Scale())
	gaussianBlur(cover, w, h, sigma)

	r, g, b, a := shadow.Color.RGBA()
	if a == 0 {
		return
	}
	// RGBA is premultiplied; the image is not.
	nr, ng, nb := uint8(r*0xffff/a>>8), uint8(g*0xffff/a>>8), uint8(b*0xffff/a>>8)
	alpha := float32(a) / 0xffff
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i, v := range cover {
		if v <= 0 {
			continue
		}
		p := img.Pix[i*4 : i*4+4 : i*4+4]
		p[0], p[1], p[2] = nr, ng, nb
		p[3] = uint8(min(v*alpha, 1)*0xff + 0.5)
	}
	draw.Draw(base, out, img, out.Min.Sub(image.Pt(x, y)), draw.Over)
}

// gaussianBlur blurs a w×h plane in place by a Gaussian of sigma pixels: once
// along each axis, with the kernel cut at 3 sigma and renormalised, and the
// plane taken as zero outside its bounds.
func gaussianBlur(plane []float32, w, h int, sigma float64) {
	if sigma <= 0 {
		return
	}
	radius := int(math.Ceil(3 * sigma))
	kernel := make([]float32, 2*radius+1)
	var sum float64
	for i := range kernel {
		d := float64(i - radius)
		v := math.Exp(-d * d / (2 * sigma * sigma))
		kernel[i] = float32(v)
		sum += v
	}
	for i := range kernel {
		kernel[i] /= float32(sum)
	}

	tmp := make([]float32, max(w, h))
	blurLine := func(get func(int) float32, set func(int, float32), n int) {
		for i := 0; i < n; i++ {
			var acc float32
			for k, kv := range kernel {
				if j := i + k - radius; j >= 0 && j < n {
					acc += get(j) * kv
				}
			}
			tmp[i] = acc
		}
		for i := 0; i < n; i++ {
			set(i, tmp[i])
		}
	}
	for row := 0; row < h; row++ {
		line := plane[row*w : row*w+w]
		blurLine(func(i int) float32 { return line[i] }, func(i int, v float32) { line[i] = v }, w)
	}
	for col := 0; col < w; col++ {
		blurLine(func(i int) float32 { return plane[i*w+col] }, func(i int, v float32) { plane[i*w+col] = v }, h)
	}
}
