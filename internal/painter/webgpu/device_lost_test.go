//go:build !gl && !wasm

// (!wasm: these acquire a real WebGPU device; see canvas_fbo_test.go.)

package webgpu

import (
	"errors"
	"testing"

	"github.com/belluikit/webgpu/wgpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBeginFrameOnALiveDeviceAcquires is the control for the lost-device
// test below: a frame begun on a live device does reach the surface acquire.
func TestBeginFrameOnALiveDeviceAcquires(t *testing.T) {
	o := newOffscreenPainter(t)
	err := o.dev.BeginFrame(wgpu.Color{})
	assert.ErrorIs(t, err, errNoSurface, "the offscreen device has no surface")
	assert.Equal(t, 1, o.dev.acquires)
}

// TestLostDeviceIsDroppedBeforeAcquire pins #1150: on a lost device
// wgpu-native aborts the process in the surface acquire, so a frame must find
// the loss first, and the painter must let go of the device and of everything
// made with it.
func TestLostDeviceIsDroppedBeforeAcquire(t *testing.T) {
	o := newOffscreenPainter(t)
	tex, err := o.dev.CreateRenderTexture(8, 8)
	require.NoError(t, err)
	o.registerTexture(tex)
	d := o.dev

	d.device.Destroy()
	err = d.BeginFrame(wgpu.Color{})
	assert.True(t, errors.Is(err, ErrDeviceLost), "BeginFrame on a lost device: %v", err)
	assert.Equal(t, 0, d.acquires, "the acquire is never reached")

	o.Clear()
	assert.Nil(t, o.dev, "the lost device is dropped")
	assert.Empty(t, o.textures, "with the textures made on it")
	assert.False(t, o.framing)
	assert.Equal(t, 0, d.acquires)

	o.Clear() // no surface descriptor: no new device, and nothing to draw on
	assert.Nil(t, o.dev)
}
