//go:build !gl && !wasm && !race

// (!wasm: acquires a real WebGPU device, see arena_readback_test.go. !race:
// the race detector's shadow memory grows the resident size by tens of MB on
// its own, which hides the leak this measures. The CI steps that run this
// package's GPU tests do not use -race.)

package webgpu

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/belluikit/webgpu/wgpu"
	"github.com/stretchr/testify/require"
)

// TestSubmitFrameReclaimsDestroyedTextures guards #452: frames that render into
// short-lived targets and free them with destroyTexture, as box shadows do,
// must not grow native memory. wgpu-core frees a destroyed texture's views only
// when the device is polled, so without submitFrame's poll every destroyed
// target leaked for the life of the window.
func TestSubmitFrameReclaimsDestroyedTextures(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("reads the process's resident size with ps")
	}
	instance := wgpu.CreateInstance(chooseInstanceBackends())
	defer instance.Release()
	adapter, err := instance.RequestAdapter(nil)
	if err != nil {
		t.Skipf("no WebGPU adapter available: %v", err)
	}
	defer adapter.Release()
	device, err := adapter.RequestDevice(nil)
	if err != nil {
		t.Skipf("no WebGPU device available: %v", err)
	}

	// d owns device and queue from here: Destroy releases them.
	d := &Device{device: device, queue: device.GetQueue(), arenaMaxUni: uniformBlockStride}
	defer d.Destroy()
	require.NoError(t, d.buildLayouts())
	require.NoError(t, d.ensureArenas())
	d.sampler, err = device.TryCreateSampler(nil)
	require.NoError(t, err)

	const targetsPerFrame = 100
	frame := func() {
		enc, err := device.TryCreateCommandEncoder(nil)
		require.NoError(t, err)
		var targets [targetsPerFrame]*wgpu.Texture
		var binds [targetsPerFrame]*wgpu.BindGroup
		for i := range targets {
			tex, err := device.TryCreateTexture(&wgpu.TextureDescriptor{
				Size:          wgpu.Extent3D{Width: 16, Height: 16, DepthOrArrayLayers: 1},
				MipLevelCount: 1,
				SampleCount:   1,
				Dimension:     wgpu.TextureDimension2D,
				Format:        wgpu.TextureFormatRGBA8Unorm,
				Usage:         wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
			})
			require.NoError(t, err)
			view, err := tex.TryCreateView(nil)
			require.NoError(t, err)
			pass := enc.BeginRenderPass(&wgpu.RenderPassDescriptor{
				ColorAttachments: []wgpu.RenderPassColorAttachment{{View: view, LoadOp: wgpu.LoadOpClear, StoreOp: wgpu.StoreOpStore}},
			})
			pass.End()
			pass.Release()
			// A bind group sampling the target, as compositing a shadow does.
			bind, err := device.TryCreateBindGroup(&wgpu.BindGroupDescriptor{
				Layout: d.bglTextured,
				Entries: []wgpu.BindGroupEntry{
					{Binding: 0, Buffer: d.uniArena, Size: uniformBlockStride},
					{Binding: 1, TextureView: view},
					{Binding: 2, Sampler: d.sampler},
				},
			})
			require.NoError(t, err)
			view.Release()
			targets[i], binds[i] = tex, bind
		}
		cmd, err := enc.TryFinish(nil)
		require.NoError(t, err)
		d.submitFrame(cmd)
		cmd.Release()
		enc.Release()
		for i, tex := range targets {
			binds[i].Release()
			destroyTexture(tex)
		}
		// Give the GPU time to finish the frame, as a display's refresh does.
		time.Sleep(time.Millisecond)
	}

	// Resident size first climbs to a plateau while Metal and the allocator
	// keep freed memory for reuse (~1000 frames on an M3 Max), so measure past
	// it. From there it grows about 2 MB per 1000 frames with the poll, and
	// grows ~60 MB without it.
	for range 1000 {
		frame()
	}
	before := residentBytes(t)
	for range 1000 {
		frame()
	}
	grown := float64(residentBytes(t)) - float64(before)
	t.Logf("resident size grew %.1f MB over 1000 frames", grown/(1<<20))
	require.Less(t, grown, float64(8<<20), "destroyed targets are not being freed")
}

// residentBytes is the process's resident set size.
func residentBytes(t *testing.T) uint64 {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(os.Getpid())).Output()
	require.NoError(t, err)
	kb, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	require.NoError(t, err)
	return kb * 1024
}
