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
	"slices"
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
	}

	// The frames must not outrun the GPU. Until the GPU finishes a frame, its
	// destroyed targets stay allocated however often the device is polled, so
	// a loop that submits faster than the GPU completes reads the growing
	// backlog as a leak. That happened on a Mac whose GPU was shared with
	// virtual machines (#461). So time the GPU first and pace each frame to
	// twice that. The pacing sleeps rather than polls: a poll here would free
	// what a missing poll in submitFrame leaks, and hide the bug.
	timeGPU := func() time.Duration {
		start := time.Now()
		for range 20 {
			frame()
			d.device.Poll(true, nil)
		}
		return time.Since(start) / 20
	}
	gpuFrame := timeGPU()
	if gpuFrame > 25*time.Millisecond {
		t.Skipf("the GPU takes %v per frame here; paced, the test would run for minutes", gpuFrame)
	}
	pace := max(2*gpuFrame, time.Millisecond)

	// Resident size first climbs to a plateau while Metal and the allocator
	// keep freed memory for reuse, so measure past it. From there it grows
	// under 1 MB per 500 frames with the poll, and about 34 MB without it.
	//
	// A leak grows every window by the same amount; what Metal and the
	// allocator keep after heavy GPU use, such as a perfscenes loop just
	// before, moves the resident size by up to 12 MB in one window and not
	// the next (#478). So the test judges the median of several windows.
	for range 300 {
		frame()
		time.Sleep(pace)
	}
	const windows, framesPerWindow = 5, 200
	growth := make([]float64, windows)
	for w := range growth {
		before := residentBytes(t)
		for range framesPerWindow {
			frame()
			time.Sleep(pace)
		}
		growth[w] = (float64(residentBytes(t)) - float64(before)) / (1 << 20)
	}
	t.Logf("resident size grew %.1f MB per %d frames, paced at %v", growth, framesPerWindow, pace)
	slices.Sort(growth)
	if growth[windows/2] < 4.0 {
		return
	}
	// A GPU that slowed during the test past the pacing was outrun, and its
	// backlog of destroyed targets reads as a leak: a CI runner's shared GPU
	// held 231 MB after 500 frames (#478). Timing it again is safe now, after
	// the readings, and tells the two apart. Waiting for the GPU before
	// reading cannot: a poll frees what a missing poll in submitFrame leaks.
	now := timeGPU()
	if now > pace {
		t.Skipf("the GPU slowed from %v to %v per frame during the test, past the pacing of %v: "+
			"the growth (median %.1f MB) is its backlog, not necessarily a leak", gpuFrame, now, pace, growth[windows/2])
	}
	t.Fatalf("destroyed targets are not being freed: median %.1f MB per window (limit 4), "+
		"the GPU taking %v per frame before and %v after", growth[windows/2], gpuFrame, now)
}

// residentBytes is the process's resident set size.
func residentBytes(t *testing.T) uint64 {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(os.Getpid())).Output()
	require.NoError(t, err)
	kb, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	require.NoError(t, err)
	return kb * 1024
}
