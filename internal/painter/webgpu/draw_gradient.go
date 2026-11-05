package webgpu

import (
	"image/color"
	"math"
	"sort"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// maxGradientStops is how many stops shaders/gradient.wgsl takes: its uniform
// block is then 256 bytes, the most a block may be (uniformBlockStride). A
// gradient with more is drawn from its CPU texture, as before #956.
const maxGradientStops = 6

// gpuGradients selects the gradient shader; tests turn it off to draw the
// CPU texture reference.
var gpuGradients = true

// drawGradient draws a LinearGradient or RadialGradient with the gradient
// shader (#956): the colour is computed per fragment, so a gradient whose
// colours or size change costs no CPU image and no texture upload. It
// mirrors the objects' Generate, the reference the texture path draws.
// It reports false, drawing nothing, for a gradient the shader cannot take.
func (p *painter) drawGradient(o core.CanvasObject, pos geo.Position, frame geo.Size) bool {
	var (
		kind   float32
		linear [4]float32
		radial [4]float32
		stops  []canvas.GradientStop
		start  color.Color
		end    color.Color
	)
	size := o.Size()
	w := p.textureScale(size.Width)
	h := p.textureScale(size.Height)
	if w <= 0 || h <= 0 {
		return true // Generate would draw nothing
	}

	switch g := o.(type) {
	case *canvas.LinearGradient:
		linear = linearGradientForm(g.Angle, w, h)
		stops, start, end = g.Stops, g.StartColor, g.EndColor
	case *canvas.RadialGradient:
		kind = 1
		radial = radialGradientForm(g.CenterOffsetX, g.CenterOffsetY, w, h)
		stops, start, end = g.Stops, g.StartColor, g.EndColor
	default:
		return false
	}

	var pos8 [8]float32
	var col8 [maxGradientStops][4]float32
	n := 0
	if len(stops) > 0 {
		if len(stops) > maxGradientStops {
			return false
		}
		sorted := make([]canvas.GradientStop, len(stops))
		copy(sorted, stops)
		for i := range sorted {
			sorted[i].Position = math.Max(0, math.Min(1, sorted[i].Position))
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Position < sorted[j].Position })
		for i, s := range sorted {
			pos8[i] = float32(s.Position)
			col8[i] = premulColor(s.Color)
		}
		n = len(sorted)
		if n == 1 { // one stop paints the whole object its colour
			pos8[1], col8[1] = pos8[0], col8[0]
			n = 2
		}
	} else {
		if start == nil && end == nil {
			return true // Generate leaves the image transparent
		}
		if start == nil {
			start = color.Transparent
		}
		if end == nil {
			end = color.Transparent
		}
		pos8[0], col8[0] = 0, premulColor(start)
		pos8[1], col8[1] = 1, premulColor(end)
		n = 2
	}

	bounds, points := p.shapeQuad(o, pos, frame, 0)
	fw, fh := p.scaleFrameSize(frame)
	x1, _, y1, _ := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	u := GradientUniforms{
		FrameSize:    [2]float32{fw, fh},
		Size:         [2]float32{w, h},
		Origin:       [2]float32{x1, fh - y1},
		Kind:         kind,
		StopCount:    float32(n),
		Linear:       linear,
		Radial:       radial,
		Alpha:        p.effectiveAlpha(1),
		InvTransform: p.invTransformStd140(pos, frame),
		StopPos:      pos8,
		StopCol:      col8,
	}
	p.record("gradient", points, 4, uniformBytes(&u), nil)
	return true
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
