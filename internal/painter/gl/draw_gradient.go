//go:build gl || (!darwin && !linux && !windows && !wasm)

package gl

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	paint "github.com/belluikit/bellui/internal/painter"
)

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
// the WebGPU painter's drawGradient does (#956); both take the shader's inputs
// from paint.GradientShaderParams (#963). It reports false, drawing nothing,
// for a gradient the shader cannot take.
func (p *painter) drawGradientShader(o core.CanvasObject, pos geo.Position, frame geo.Size) bool {
	if !gpuGradients || p.gradientProgram.uniforms == nil {
		return false
	}
	size := o.Size()
	w := p.textureScale(size.Width)
	h := p.textureScale(size.Height)
	g, draw, ok := paint.GradientShaderParams(o, w, h)
	if !ok {
		return false
	}
	if !draw {
		return true
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
	p.SetUniform4f(program, "params", g.Kind, float32(g.StopCount), p.effectiveAlpha(1), 0)
	p.SetUniform4f(program, "linear", g.Linear[0], g.Linear[1], g.Linear[2], g.Linear[3])
	p.SetUniform4f(program, "radial", g.Radial[0], g.Radial[1], g.Radial[2], g.Radial[3])
	p.SetUniform4f(program, "stop_pos[0]", g.StopPos[0], g.StopPos[1], g.StopPos[2], g.StopPos[3])
	p.SetUniform4f(program, "stop_pos[1]", g.StopPos[4], g.StopPos[5], g.StopPos[6], g.StopPos[7])
	for i, c := range g.StopCol {
		p.SetUniform4f(program, stopColUniforms[i], c[0], c[1], c[2], c[3])
	}
	p.logError()

	p.ctx.DrawArrays(triangleStrip, 0, 4)
	p.logError()
	return true
}
