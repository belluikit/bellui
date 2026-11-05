//go:build !gl

package webgpu

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	paint "github.com/belluikit/bellui/internal/painter"
)

// gpuGradients selects the gradient shader; tests turn it off to draw the
// CPU texture reference.
var gpuGradients = true

// drawGradient draws a LinearGradient or RadialGradient with the gradient
// shader (#956): the colour is computed per fragment, so a gradient whose
// colours or size change costs no CPU image and no texture upload. The
// shader's inputs come from paint.GradientShaderParams, shared with the GL
// painter (#963). It reports false, drawing nothing, for a gradient the
// shader cannot take.
func (p *painter) drawGradient(o core.CanvasObject, pos geo.Position, frame geo.Size) bool {
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

	bounds, points := p.shapeQuad(o, pos, frame, 0)
	fw, fh := p.scaleFrameSize(frame)
	x1, _, y1, _ := p.scaleRectCoords(bounds[0], bounds[2], bounds[1], bounds[3])
	u := GradientUniforms{
		FrameSize:    [2]float32{fw, fh},
		Size:         [2]float32{w, h},
		Origin:       [2]float32{x1, fh - y1},
		Kind:         g.Kind,
		StopCount:    float32(g.StopCount),
		Linear:       g.Linear,
		Radial:       g.Radial,
		Alpha:        p.effectiveAlpha(1),
		InvTransform: p.invTransformStd140(pos, frame),
		StopPos:      g.StopPos,
		StopCol:      g.StopCol,
	}
	p.record("gradient", points, 4, uniformBytes(&u), nil)
	return true
}
