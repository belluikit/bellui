//go:build !gl && !wasm

// (!wasm: these acquire a real WebGPU device; see canvas_fbo_test.go.)

package webgpu

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A frame that can't be opened (the surface occluded, as while a window is
// first shown) draws nothing into the canvas FBO. With partial repaint the
// driver's bookkeeping moves on regardless, so the next frame must clear and
// redraw the FBO whole; otherwise a frame that "cleared" nothing, or a partial
// frame that drew nothing, left the content missing until something changed:
// TestWindowedSurfaceCapture read the background 2-3 runs in 20 (#595).
func TestCanvasFBOFullClearAfterFrameThatDidNotOpen(t *testing.T) {
	o := newOffscreenPainter(t) // its device has no surface, so BeginFrame fails
	o.EnsureCanvasFBO(clipDim, clipDim)
	require.True(t, o.CanvasFBONeedsFullClear(), "a new FBO needs a full clear")

	o.BindCanvasFBO()
	require.False(t, o.framing, "no surface: the frame does not open")
	o.MarkCanvasFBOCleared()
	assert.True(t, o.CanvasFBONeedsFullClear(), "a frame that did not open cleared nothing")

	// The FBO was cleared in an earlier frame, and now a frame fails.
	o.canvasFBO.initialClear = true
	o.BindCanvasFBO()
	assert.True(t, o.CanvasFBONeedsFullClear(), "after a frame that did not open, the next must be full")
}
