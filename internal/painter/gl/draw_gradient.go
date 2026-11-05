//go:build gl || (!darwin && !linux && !windows && !wasm)

package gl

import (
	"image/color"
	"math"
	"sort"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// maxGradientStops is how many stops shaders/gradient.frag takes. A gradient
// with more is drawn from its CPU texture, as before #957.
const maxGradientStops = 6

// gpuGradients selects the gradient shader; tests turn it off to draw the
// CPU texture reference.
var gpuGradients = true

// gradientUniforms names the gradient program's uniforms, the stop arrays one
// element at a time: GL ES 2 sets an array element through its own location.
var gradientUniforms = []string{
	"geom", "params", "linear", "radial",
	"stop_pos[0]", "stop_pos[1]",
	"stop_col[0]", "stop_col[1]", "stop_col[2]", "stop_col[3]", "stop_col[4]", "stop_col[5]",
}

var stopColUniforms = gradientUniforms[6:]

// initGradientProgram builds the gradient program. Unlike the other
// programs, a failure here does not degrade the painter: gradients keep
// their CPU texture, which drew them before the shader existed.
func (p *painter) initGradientProgram(name string) {
	ref, reason, err := p.buildProgram(name)
	if err != nil {
		bellui.LogError("GL gradient shader "+name+" unavailable ("+reason+"); gradients are drawn from a CPU image", err)
		p.gradientProgram = ProgramState{}
		return
	}
	p.gradientProgram = ProgramState{
		ref:            ref,
		buff:           p.createBuffer(16),
		uniforms:       make(map[string]*UniformState),
		matrixUniforms: make(map[string]*MatrixUniformState),
		attributes:     make(map[string]Attribute),
	}
	p.getUniformLocations(p.gradientProgram, gradientUniforms...)
	p.getMatrixUniformLocations(p.gradientProgram, "inv_transform")
	p.enableAttribArrays(p.gradientProgram, "vert", "normal")
}

// drawGradientShader draws a LinearGradient or RadialGradient with the
// gradient shader (#957): the colour is computed per fragment, so a gradient
// whose colours or size change costs no CPU image and no texture upload. It
// mirrors the objects' Generate, the reference the texture path draws, as
// the WebGPU painter's drawGradient does (#956). It reports false, drawing
// nothing, for a gradient the shader cannot take.
func (p *painter) drawGradientShader(o core.CanvasObject, pos geo.Position, frame geo.Size) bool {
	if !gpuGradients || p.gradientProgram.uniforms == nil {
		return false
	}
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
	var col6 [maxGradientStops][4]float32
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
			col6[i] = premulColor(s.Color)
		}
		n = len(sorted)
		if n == 1 { // one stop paints the whole object its colour
			pos8[1], col6[1] = pos8[0], col6[0]
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
		pos8[0], col6[0] = 0, premulColor(start)
		pos8[1], col6[1] = 1, premulColor(end)
		n = 2
	}

	program := p.gradientProgram
	var bounds [4]float32
	var points []float32
	if p.hasTransform() {
		bounds, _ = p.vecRectCoords(pos, o, frame, 0)
		_, points = p.transformedAABBQuad(pos, size, frame)
	} else {
		bounds, points = p.vecRectCoords(pos, o, frame, 0)
	}
	p.ctx.UseProgram(program.ref)
	p.updateBuffer(program.buff, points)
	p.UpdateVertexArray(program, "vert", 2, 4, 0)
	p.UpdateVertexArray(program, "normal", 2, 4, 2)
	p.blendFunc(one, oneMinusSrcAlpha)
	p.logError()

	p.uploadInvTransform(program, pos, frame)
	_, fh := p.scaleFrameSize(frame)
	x1, _, y1, _ := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	p.SetUniform4f(program, "geom", w, h, x1, fh-y1)
	p.SetUniform4f(program, "params", kind, float32(n), p.effectiveAlpha(1), 0)
	p.SetUniform4f(program, "linear", linear[0], linear[1], linear[2], linear[3])
	p.SetUniform4f(program, "radial", radial[0], radial[1], radial[2], radial[3])
	p.SetUniform4f(program, "stop_pos[0]", pos8[0], pos8[1], pos8[2], pos8[3])
	p.SetUniform4f(program, "stop_pos[1]", pos8[4], pos8[5], pos8[6], pos8[7])
	for i, c := range col6 {
		p.SetUniform4f(program, stopColUniforms[i], c[0], c[1], c[2], c[3])
	}
	p.logError()

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
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
