//go:build !gl

package webgpu

import (
	"github.com/belluikit/webgpu/wgpu"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/gpu"
)

// RepaintBoundary FBO caching for WebGPU (pre-rebrand tracker #202), implementing gpu.BoundaryPainter.
// Mirrors internal/painter/gl/boundary.go, but maps onto the Device's
// render-target stack: binding a boundary pushes its texture as the render
// target (a fresh pass cleared to transparent), unbinding pops back to the
// parent (re-opened with LoadOp=Load), and the blit samples the boundary
// texture into the parent pass. Unlike GL, render-to-texture is top-down here,
// so no V-flip is needed on the blit.

// boundaryState is a RepaintBoundary's cached off-screen target. Implements
// gpu.BoundaryFBO.
type boundaryState struct {
	tex     *Texture
	width   int
	height  int
	logical geo.Size
	dirty   bool
}

// SetDirty implements gpu.BoundaryFBO.
func (s *boundaryState) SetDirty(dirty bool) { s.dirty = dirty }

func (p *painter) EnsureBoundaryFBO(obj core.CanvasObject, pixW, pixH int, logicalSize geo.Size) gpu.BoundaryFBO {
	// The device is brought up lazily (mobile builds its surface from the native
	// window, which may not exist on the first paint — painter.Init/SetOutputSize
	// then no-op). Until it is, return a no-op state (nil tex): Bind/Unbind/Blit
	// all skip it, so the RepaintBoundary subtree just isn't cached this frame
	// rather than dereferencing a nil device (pre-rebrand tracker #34). Once the device is up, the
	// next paint creates the real FBO. Mirrors the p.dev==nil guard on the paint
	// ops (painter.go) and the canvas FBO path (canvas_fbo.go).
	if p.dev == nil {
		return &boundaryState{logical: logicalSize}
	}
	if p.boundaries == nil {
		p.boundaries = map[core.CanvasObject]*boundaryState{}
	}
	if s, ok := p.boundaries[obj]; ok && s.width == pixW && s.height == pixH {
		s.logical = logicalSize
		return s
	} else if ok {
		// Resizing: the old target may already be sampled by this frame's
		// commands, so free it once the frame is submitted.
		p.dev.ReleaseTextureAfterFrame(s.tex)
		delete(p.boundaries, obj)
	}

	tex, err := p.dev.CreateRenderTexture(pixW, pixH)
	if err != nil {
		return &boundaryState{logical: logicalSize} // nil tex; Bind/Blit no-op safely
	}
	s := &boundaryState{tex: tex, width: pixW, height: pixH, logical: logicalSize, dirty: true}
	p.boundaries[obj] = s
	return s
}

func (p *painter) BindBoundaryFBO(state gpu.BoundaryFBO) {
	s := state.(*boundaryState)
	if p.dev == nil || s.tex == nil {
		return
	}
	p.dev.PushTarget(s.tex, s.width, s.height)
}

func (p *painter) UnbindBoundaryFBO(canvasPixW, canvasPixH int) {
	if p.dev == nil {
		return
	}
	p.dev.PopTarget()
}

func (p *painter) BlitBoundaryTexture(state gpu.BoundaryFBO, pos geo.Position, frame geo.Size) {
	s := state.(*boundaryState)
	if s.tex == nil {
		return
	}
	// The boundary quad is drawn in canvas space; reset the world transform.
	p.transform = geo.Identity()
	p.blitTexture(s.tex.View(), s.logical, pos, frame)
}

// blitTexture draws a texture view as a stretched quad at pos via the simple
// shader (no corner radius, full alpha).
func (p *painter) blitTexture(view *wgpu.TextureView, logical geo.Size, pos geo.Position, frame geo.Size) {
	points, insets := p.rectCoords(logical, pos, frame, canvas.ImageFillStretch, 0, 0)
	u := SimpleUniforms{Alpha: 1, Inset: insets}
	p.record("simple", points, 4, uniformBytes(&u), view)
}

func (p *painter) FreeBoundaryFBO(obj core.CanvasObject) {
	if p.boundaries == nil {
		return
	}
	if s, ok := p.boundaries[obj]; ok {
		if s.tex != nil {
			s.tex.Release()
		}
		delete(p.boundaries, obj)
	}
}

func (p *painter) MarkBoundaryDirty(obj core.CanvasObject) {
	if s, ok := p.boundaries[obj]; ok {
		s.dirty = true
	}
}

func (p *painter) IsBoundaryClean(obj core.CanvasObject) bool {
	s, ok := p.boundaries[obj]
	return ok && !s.dirty
}

func (p *painter) InvalidateAllBoundaries() {
	for _, s := range p.boundaries {
		s.dirty = true
	}
}
