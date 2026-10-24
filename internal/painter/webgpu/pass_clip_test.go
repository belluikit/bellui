//go:build !gl && !wasm

// (!wasm: acquires a real WebGPU device, see arena_readback_test.go.)

package webgpu

import (
	"testing"

	"github.com/belluikit/webgpu/wgpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPassClipTracksAppliedScissor follows the clip each open pass applies
// through the render-target stack, which the #434 shadow cull answers from: a
// pass opened by a push runs unclipped even while an enclosing scissor is
// remembered, and a pop re-applies that scissor.
func TestPassClipTracksAppliedScissor(t *testing.T) {
	instance := wgpu.CreateInstance(chooseInstanceBackends())
	defer instance.Release()
	adapter, err := instance.RequestAdapter(nil)
	if err != nil {
		noGPU(t, "no WebGPU adapter available: %v", err)
	}
	defer adapter.Release()
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		noGPU(t, "no WebGPU device available: %v", err)
	}

	// d owns device and queue from here: Destroy releases them.
	d := &Device{
		device:        device,
		queue:         device.GetQueue(),
		depthStencils: map[[2]int]*depthStencil{},
		config:        &wgpu.SurfaceConfiguration{Format: wgpu.TextureFormatRGBA8Unorm},
	}
	defer d.Destroy()
	root, err := d.CreateRenderTexture(400, 300)
	require.NoError(t, err)
	defer root.Release()
	off, err := d.CreateRenderTexture(100, 50)
	require.NoError(t, err)
	defer off.Release()

	d.encoder, err = device.TryCreateCommandEncoder(nil)
	require.NoError(t, err)
	d.targets = append(d.targets[:0], renderTarget{view: root.View(), tex: root.tex, w: 400, h: 300})
	d.beginTopPass()
	assert.Equal(t, scissorRect{w: 400, h: 300, active: true}, d.passClip, "a fresh pass covers its target")

	d.SetScissor(10, 20, 100, 50)
	assert.Equal(t, scissorRect{x: 10, y: 20, w: 100, h: 50, active: true}, d.passClip)
	assert.True(t, d.Culls(200, 200, 10, 10))

	d.PushOffscreenTarget(off, 100, 50, wgpu.Color{})
	assert.Equal(t, scissorRect{w: 100, h: 50, active: true}, d.passClip, "a pushed pass runs unclipped")
	assert.False(t, d.Culls(60, 30, 10, 10), "the enclosing scissor does not apply off-screen")

	d.PopTarget()
	assert.Equal(t, scissorRect{x: 10, y: 20, w: 100, h: 50, active: true}, d.passClip, "a pop re-applies the scissor")

	d.ClearScissor()
	assert.Equal(t, scissorRect{w: 400, h: 300, active: true}, d.passClip)

	d.endPass()
	cmd, err := d.encoder.TryFinish(nil)
	require.NoError(t, err)
	cmd.Release()
	d.encoder.Release()
	d.encoder = nil
	for _, ds := range d.frameDepths {
		ds.Release()
	}
	d.frameDepths = nil
}
