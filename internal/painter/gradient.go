package painter

import (
	"image/color"
	"math"
	"sort"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
)

// MaxGradientStops is how many stops the GPU painters' gradient shaders take.
// The WebGPU uniform block is then 256 bytes, the most a block may be, and
// the GL shader's stop arrays are sized to match. A gradient with more is
// drawn from its CPU texture.
const MaxGradientStops = 6

// GradientParams is a LinearGradient or RadialGradient in the form the GPU
// painters' gradient shaders take (#956, #957). It mirrors the objects'
// Generate, which stays the reference.
type GradientParams struct {
	// Kind is 0 for a linear gradient and 1 for a radial one.
	Kind float32
	// Linear is Generate's linear distance as |kx*x + ky*y + k0| / den.
	Linear [4]float32
	// Radial is Generate's centre and semi-axes.
	Radial [4]float32
	// StopPos holds the sorted stop positions, clamped to [0, 1]; only the
	// first StopCount are used.
	StopPos [8]float32
	// StopCol holds each stop's colour, premultiplied RGBA in [0, 1].
	StopCol [MaxGradientStops][4]float32
	// StopCount is at least 2: a single stop is padded to two.
	StopCount int
}

// GradientShaderParams computes the shader form of o, a gradient drawn into
// an image w by h pixels. ok is false when the shader cannot take o (not a
// gradient, or more than MaxGradientStops stops): the caller then draws the
// CPU texture. draw is false when Generate would leave the image transparent,
// so there is nothing to draw.
func GradientShaderParams(o core.CanvasObject, w, h float32) (p GradientParams, draw, ok bool) {
	if w <= 0 || h <= 0 {
		return p, false, true
	}
	var (
		stops      []canvas.GradientStop
		start, end color.Color
	)
	switch g := o.(type) {
	case *canvas.LinearGradient:
		p.Linear = linearGradientForm(g.Angle, w, h)
		stops, start, end = g.Stops, g.StartColor, g.EndColor
	case *canvas.RadialGradient:
		p.Kind = 1
		p.Radial = radialGradientForm(g.CenterOffsetX, g.CenterOffsetY, w, h)
		stops, start, end = g.Stops, g.StartColor, g.EndColor
	default:
		return p, false, false
	}

	if len(stops) == 0 {
		if start == nil && end == nil {
			return p, false, true
		}
		if start == nil {
			start = color.Transparent
		}
		if end == nil {
			end = color.Transparent
		}
		p.StopPos[0], p.StopCol[0] = 0, premulColor(start)
		p.StopPos[1], p.StopCol[1] = 1, premulColor(end)
		p.StopCount = 2
		return p, true, true
	}

	if len(stops) > MaxGradientStops {
		return p, false, false
	}
	sorted := make([]canvas.GradientStop, len(stops))
	copy(sorted, stops)
	for i := range sorted {
		sorted[i].Position = math.Max(0, math.Min(1, sorted[i].Position))
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Position < sorted[j].Position })
	for i, s := range sorted {
		p.StopPos[i] = float32(s.Position)
		p.StopCol[i] = premulColor(s.Color)
	}
	p.StopCount = len(sorted)
	if p.StopCount == 1 { // one stop paints the whole object its colour
		p.StopPos[1], p.StopCol[1] = p.StopPos[0], p.StopCol[0]
		p.StopCount = 2
	}
	return p, true, true
}

// linearGradientForm is LinearGradient.Generate's distance as
// |kx*x + ky*y + k0| / den, for an image w by h pixels.
func linearGradientForm(angle float64, w, h float32) [4]float32 {
	switch angle {
	case 90, -270: // (w - x) / w
		return [4]float32{-1, 0, w, w}
	case 270, -90: // x / w
		return [4]float32{1, 0, 0, w}
	case 45, -315: // |w - x + y| / (w + h)
		return [4]float32{-1, 1, w, w + h}
	case 225, -135: // |x + h - y| / (w + h)
		return [4]float32{1, -1, h, w + h}
	case 135, -225: // |w + h - (x + y)| / (w + h)
		return [4]float32{-1, -1, w + h, w + h}
	case 315, -45: // |x + y| / (w + h)
		return [4]float32{1, 1, 0, w + h}
	case 180, -180: // (h - y) / h
		return [4]float32{0, -1, h, h}
	default: // y / h
		return [4]float32{0, 1, 0, h}
	}
}

// radialGradientForm is RadialGradient.Generate's centre and semi-axes for an
// image w by h pixels.
func radialGradientForm(offX, offY float64, w, h float32) [4]float32 {
	cx := float64(w)/2 + float64(w)*offX
	cy := float64(h)/2 + float64(h)*offY
	a, b := cx, cy
	if offX < 0 {
		a = float64(w) - cx
	}
	if offY < 0 {
		b = float64(h) - cy
	}
	return [4]float32{float32(cx), float32(cy), float32(a), float32(b)}
}

// premulColor is c as premultiplied RGBA in [0, 1], the space Generate
// interpolates in.
func premulColor(c color.Color) [4]float32 {
	if c == nil {
		return [4]float32{}
	}
	r, g, b, a := c.RGBA()
	return [4]float32{float32(r) / 0xffff, float32(g) / 0xffff, float32(b) / 0xffff, float32(a) / 0xffff}
}
