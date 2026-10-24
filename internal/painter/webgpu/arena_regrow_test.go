//go:build !gl && !wasm

// (!wasm: acquires a real WebGPU device, see arena_readback_test.go.)

package webgpu

import (
	"testing"

	"github.com/belluikit/webgpu/wgpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnsureArenasFailedRegrowKeepsOldBuffer guards #436: when an arena must
// grow and creating the larger buffer fails (a lost device in the field, here a
// size past the adapter's maxBufferSize), the old arena must stay in place and
// usable. The bug released the old buffer first and kept the released wrapper,
// so the next frame's ensureArenas destroyed it again and wgpu-native aborted
// the process ("invalid buffer").
func TestEnsureArenasFailedRegrowKeepsOldBuffer(t *testing.T) {
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
	queue := device.GetQueue()

	// d owns device and queue from here: Destroy releases them.
	d := &Device{
		device:      device,
		queue:       queue,
		arenaMaxVtx: 64 * 1024,
		arenaMaxUni: 256 * uniformBlockStride,
	}
	defer d.Destroy()
	require.NoError(t, d.buildLayouts())
	require.NoError(t, d.ensureArenas())
	vtx, vtxCap := d.vtxArena, d.vtxArenaCap
	uni, uniCap := d.uniArena, d.uniArenaCap
	bind := d.arenaBindUniform

	tooBig := int(adapter.GetLimits().MaxBufferSize) + 1
	d.arenaMaxVtx, d.arenaMaxUni = tooBig, tooBig

	// Two frames in a row: the second is the one that aborted.
	for frame := range 2 {
		assert.Error(t, d.ensureArenas(), "frame %d: a regrow past maxBufferSize must fail", frame)
		assert.Same(t, vtx, d.vtxArena, "frame %d: the vertex arena must be kept", frame)
		assert.Equal(t, vtxCap, d.vtxArenaCap, "frame %d", frame)
	}

	// The uniform branch has the same shape; reach it with the vertex grow fitting.
	d.arenaMaxVtx = vtxCap
	for frame := range 2 {
		assert.Error(t, d.ensureArenas(), "frame %d: a regrow past maxBufferSize must fail", frame)
		assert.Same(t, uni, d.uniArena, "frame %d: the uniform arena must be kept", frame)
		assert.Equal(t, uniCap, d.uniArenaCap, "frame %d", frame)
		assert.Same(t, bind, d.arenaBindUniform, "frame %d: the bind group over the kept arena stays valid", frame)
	}

	// The kept buffers are still live: writing them and submitting is accepted.
	queue.WriteBuffer(d.vtxArena, 0, make([]byte, 256))
	queue.WriteBuffer(d.uniArena, 0, make([]byte, uniformBlockStride))
	enc, err := device.TryCreateCommandEncoder(nil)
	require.NoError(t, err)
	cmd, err := enc.TryFinish(nil)
	require.NoError(t, err)
	queue.Submit(cmd)
	cmd.Release()
	enc.Release()

	// And a later regrow that fits still succeeds.
	d.arenaMaxUni = uniCap * 2
	require.NoError(t, d.ensureArenas())
	assert.Equal(t, roundUpBuffer(uniCap*2), d.uniArenaCap)
	assert.NotNil(t, d.arenaBindUniform)
}
