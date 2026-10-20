//go:build !gl && !wasm

// (!wasm: these acquire a real WebGPU device; see canvas_fbo_test.go.)

package webgpu

import (
	"image/color"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
)

// TestBoundaryRoundedClipUsesBoundaryFrame pins #564: inside a
// RepaintBoundary, a rounded clip is stamped in the boundary's frame. Stamped
// in the canvas's, it came out scaled by the boundary's share of the canvas,
// so a Card in a boundary drew only its top-left corner.
func TestBoundaryRoundedClipUsesBoundaryFrame(t *testing.T) {
	o := newOffscreenPainter(t) // a clipDim² canvas
	const side = clipDim / 2
	logical := geo.NewSquareSize(side)
	obj := canvas.NewRectangle(nil)
	state := o.EnsureBoundaryFBO(obj, side, side, logical)
	s := state.(*boundaryState)
	require.NotNil(t, s.tex)

	o.frame(t, func() {
		o.framing = true
		defer func() { o.framing = false }()
		o.BindBoundaryFBO(state)
		o.StartRoundedClipping(geo.NewPos(4, 4), geo.NewSquareSize(side-8), 0)
		r := canvas.NewRectangle(clipRed)
		r.Resize(logical)
		o.drawRectangle(r, geo.Position{}, logical)
		o.StopRoundedClipping()
		o.UnbindBoundaryFBO(clipDim, clipDim)
	})

	img := o.dev.CaptureTexture(s.tex.tex, side, side)
	require.NotNil(t, img)
	assert.Equal(t, clipRed, pixel(img, side-6, side-6), "inside the clip, near its far corner")
	assert.Equal(t, clipRed, pixel(img, side/2, side/2), "inside the clip")
	assert.Equal(t, color.NRGBA{}, pixel(img, 1, 1), "outside the clip")
	assert.Equal(t, color.NRGBA{}, pixel(img, side-2, side-2), "outside the clip")
}

// TestBoundaryWithoutFrame pins #564's crash: a paint walk can reach a dirty
// RepaintBoundary while no frame is open (the surface was occluded, so
// BeginFrame failed). Binding it then began a render pass on a nil encoder. It
// must do nothing, and the boundary must stay dirty, or its blank texture
// would be blitted as clean from then on.
func TestBoundaryWithoutFrame(t *testing.T) {
	o := newOffscreenPainter(t)
	obj := canvas.NewRectangle(nil)
	logical := geo.NewSquareSize(16)
	state := o.EnsureBoundaryFBO(obj, 16, 16, logical)

	require.False(t, o.framing)
	assert.NotPanics(t, func() {
		o.BindBoundaryFBO(state)
		o.UnbindBoundaryFBO(clipDim, clipDim)
		state.SetDirty(false)
		o.BlitBoundaryTexture(state, geo.Position{}, geo.NewSquareSize(clipDim))
	})
	assert.False(t, o.IsBoundaryClean(obj), "a boundary nothing was drawn into must stay dirty")

	// With a frame open the same sequence renders it, and it is clean.
	o.frame(t, func() {
		o.framing = true
		defer func() { o.framing = false }()
		o.BindBoundaryFBO(state)
		o.UnbindBoundaryFBO(clipDim, clipDim)
		state.SetDirty(false)
	})
	assert.True(t, o.IsBoundaryClean(obj))
}

// TestBoundaryFreedInAnOpenFrame pins #569's second half: a boundary freed
// after it was blitted in the open frame keeps its texture until the frame is
// submitted, and the frame still draws it.
func TestBoundaryFreedInAnOpenFrame(t *testing.T) {
	o := newOffscreenPainter(t)
	obj := canvas.NewRectangle(nil)
	logical := geo.NewSquareSize(clipDim)
	state := o.EnsureBoundaryFBO(obj, clipDim, clipDim, logical)
	tex := state.(*boundaryState).tex
	require.NotNil(t, tex)

	img := o.frame(t, func() {
		o.framing = true
		defer func() { o.framing = false }()
		o.BindBoundaryFBO(state)
		o.fill()
		o.UnbindBoundaryFBO(clipDim, clipDim)
		o.BlitBoundaryTexture(state, geo.Position{}, logical)
		o.FreeBoundaryFBO(obj)
	})

	assert.Equal(t, clipRed, pixel(img, clipDim/2, clipDim/2), "the frame drew the freed boundary")
	assert.Zero(t, o.CachedBoundaries())
	assert.True(t, slices.Contains(o.dev.frameTextures, tex), "the texture waits for the frame's submit")
}

// TestBoundaryEvictedWhenUnused pins #569: a boundary no paint walk reached
// for cache.ValidDuration and boundaryKeepFrames frames is freed at the start
// of a frame, after the frame's submit, and one still drawn is kept. A
// boundary removed from the window is freed no other way.
func TestBoundaryEvictedWhenUnused(t *testing.T) {
	o := newOffscreenPainter(t)
	gone, kept := canvas.NewRectangle(nil), canvas.NewRectangle(nil)
	logical := geo.NewSquareSize(16)
	goneTex := o.EnsureBoundaryFBO(gone, 16, 16, logical).(*boundaryState).tex
	o.EnsureBoundaryFBO(kept, 16, 16, logical)
	o.boundaries[gone].used = time.Now().Add(-cache.ValidDuration - time.Second)

	o.evictBoundaries()
	assert.Equal(t, 2, o.CachedBoundaries(), "unused for a while but no frames painted: an idle app keeps its boundaries")

	o.painter.frame += boundaryKeepFrames + 1
	o.boundaries[kept].usedFrame = o.painter.frame // drawn in the latest frame
	o.evictBoundaries()

	assert.Equal(t, 1, o.CachedBoundaries())
	assert.Contains(t, o.boundaries, core.CanvasObject(kept))
	assert.True(t, slices.Contains(o.dev.frameTextures, goneTex), "the texture waits for the frame's submit")
}
