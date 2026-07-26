//go:build !gl

package webgpu

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
)

// TestBoundaryFBO_NilDevice guards the #34 crash: on mobile the WebGPU device is
// brought up lazily (its surface is built from the native window, absent on the
// first paint), so the RepaintBoundary path can run while p.dev is nil. The
// boundary painter methods must degrade to a no-op (subtree simply uncached this
// frame) instead of dereferencing the nil device.
func TestBoundaryFBO_NilDevice(t *testing.T) {
	p := &painter{} // dev == nil, as on the first mobile paint before Init succeeds
	obj := canvas.NewRectangle(nil)

	state := p.EnsureBoundaryFBO(obj, 0x550, 0x9e, geo.NewSize(100, 100))
	require.NotNil(t, state, "must return a usable (no-op) state, not nil")
	assert.Nil(t, p.boundaries, "no FBO map entry should be created without a device")

	// The full bind/unbind/blit cycle the mobile walker performs must not panic.
	assert.NotPanics(t, func() {
		p.BindBoundaryFBO(state)
		p.UnbindBoundaryFBO(0x550, 0x9e)
		p.BlitBoundaryTexture(state, geo.NewPos(0, 0), geo.NewSize(100, 100))
		p.FreeBoundaryFBO(obj)
	})
}
