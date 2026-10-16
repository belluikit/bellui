//go:build !gl

package webgpu

import (
	"os"
	"time"

	"github.com/belluikit/bellui"
)

// GPU-time profiling (pre-rebrand issue #336). When BELLUI_GPU_PROFILE is set, EndFrame forces
// the GPU to drain after presenting each frame and records the wall-clock time
// that drain took — a serialized proxy for the frame's on-GPU execution time,
// which the CPU-side paint() profiler (draw-call issuance, pipelined away from
// GPU work) cannot see. This is the measurement pre-rebrand tracker #336 needs to judge whether
// partial repaint's fill-rate savings actually pay off.
//
// It is implemented as a blocking device drain (Poll(true)) rather than
// timestamp-query readback: the wgpu binding's MapAsync passes its callback
// handle by pointer to a stack local, so the result cannot be read back safely a
// few frames later (the deferred callback dereferences a dead handle), and a
// same-frame blocking map stalled pathologically here. A drain needs no readback.
//
// The forced sync intentionally removes CPU/GPU overlap so the number isolates
// GPU work; it inflates the CPU paint+present wall time but not the separately
// measured paint() time. Use it for A/B comparison (partialrepaint on vs off),
// not as an absolute frame budget.

func gpuProfileEnabled() bool {
	return os.Getenv("BELLUI_GPU_PROFILE") != ""
}

// initGPUProfiling enables per-frame GPU drain timing from the environment.
func (d *Device) initGPUProfiling() {
	d.gpuProfile = gpuProfileEnabled()
	if d.gpuProfile {
		bellui.LogError("BELLUI_GPU_PROFILE: per-frame GPU drain timing enabled (#336)", nil)
	}
}

// gpuDrainAfterPresent blocks until the GPU finishes the just-submitted frame and
// records how long that took. Called once per frame, after Present.
func (d *Device) gpuDrainAfterPresent() {
	if !d.gpuProfile {
		return
	}
	t0 := time.Now()
	d.device.Poll(true, nil)
	d.gpuNanos = uint64(time.Since(t0).Nanoseconds())
	d.gpuValid = true
}

// GPUFrameNanos returns the most recent frame's measured GPU drain time in
// nanoseconds, and whether a valid measurement is available.
func (d *Device) GPUFrameNanos() (uint64, bool) {
	if !d.gpuProfile || !d.gpuValid {
		return 0, false
	}
	return d.gpuNanos, true
}
