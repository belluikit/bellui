//go:build !gl

package webgpu

import (
	"time"

	"github.com/belluikit/webgpu/wgpu"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
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
	// bound says the boundary is the render target this frame, so what the
	// walk draws lands in its texture. A boundary is only clean once that
	// happened: a walk with no frame open (the surface occluded, #564) draws
	// nothing, and must not leave a blank texture cached as clean.
	bound bool
	// used and usedFrame are when, and in which frame, a paint walk last
	// reached the boundary (see evictBoundaries).
	used      time.Time
	usedFrame uint64
}

// SetDirty implements gpu.BoundaryFBO. Clearing it after a render that did
// not reach the texture keeps the boundary dirty.
func (s *boundaryState) SetDirty(dirty bool) {
	if !dirty && !s.bound {
		return
	}
	s.dirty = dirty
	s.bound = false
}

// boundaryKeepFrames is how many frames a boundary no paint walk reached is
// kept for, at least (see evictBoundaries). One would do: the first frame
// after an idle spell is the one that must not evict, and its walk re-stamps
// every boundary on screen.
const boundaryKeepFrames = 3

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
		s.used, s.usedFrame = time.Now(), p.frame
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
	s := &boundaryState{tex: tex, width: pixW, height: pixH, logical: logicalSize, dirty: true, used: time.Now(), usedFrame: p.frame}
	p.boundaries[obj] = s
	return s
}

func (p *painter) BindBoundaryFBO(state gpu.BoundaryFBO) {
	s := state.(*boundaryState)
	// No open frame means no encoder to begin the boundary's pass on (#564).
	if p.dev == nil || s.tex == nil || !p.framing {
		return
	}
	p.dev.PushTarget(s.tex, s.width, s.height)
	s.bound = true
	p.boundaryFrames = append(p.boundaryFrames, s.logical)
}

func (p *painter) UnbindBoundaryFBO(canvasPixW, canvasPixH int) {
	// Pops only what BindBoundaryFBO pushed: the frame cannot open between
	// the two, so no frame here means no push.
	n := len(p.boundaryFrames)
	if p.dev == nil || n == 0 {
		return
	}
	p.boundaryFrames = p.boundaryFrames[:n-1]
	p.dev.PopTarget()
}

func (p *painter) BlitBoundaryTexture(state gpu.BoundaryFBO, pos geo.Position, frame geo.Size) {
	s := state.(*boundaryState)
	if s.tex == nil || !p.framing {
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

// FreeBoundaryFBO frees the boundary's target once the frame is submitted:
// this frame's commands may already sample it.
func (p *painter) FreeBoundaryFBO(obj core.CanvasObject) {
	if p.boundaries == nil {
		return
	}
	if s, ok := p.boundaries[obj]; ok {
		p.releaseBoundary(s)
		delete(p.boundaries, obj)
	}
}

func (p *painter) releaseBoundary(s *boundaryState) {
	if s.tex == nil {
		return
	}
	if p.dev != nil {
		p.dev.ReleaseTextureAfterFrame(s.tex)
	} else {
		s.tex.Release()
	}
}

// evictBoundaries frees the targets of the boundaries no paint walk has
// reached for cache.ValidDuration and boundaryKeepFrames frames, at the start
// of a frame. Nothing else frees a boundary that left the window:
// painter.Free is called for the objects of a refreshed subtree, which no
// longer holds it (#569). Counting frames too keeps an app that sat idle from
// dropping every boundary on screen with its first frame back.
func (p *painter) evictBoundaries() {
	if len(p.boundaries) == 0 {
		return
	}
	now := time.Now()
	for obj, s := range p.boundaries {
		if p.frame-s.usedFrame > boundaryKeepFrames && now.Sub(s.used) > cache.ValidDuration {
			p.releaseBoundary(s)
			delete(p.boundaries, obj)
		}
	}
}

// CachedBoundaries returns how many boundary targets the painter holds.
func (p *painter) CachedBoundaries() int {
	return len(p.boundaries)
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
