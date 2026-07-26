//go:build !gl

package webgpu

import (
	"errors"
	"fmt"
	"testing"

	"github.com/belluikit/webgpu/wgpu"
)

// TestClassifyAcquire locks the BeginFrame surface-acquire recovery table (#385):
// Outdated → reconfigure+retry, Lost/DeviceLost/OutOfMemory → fatal (log+skip),
// everything else (Timeout, an unexpected Success, wrapping, or a non-status error)
// → plain skip. The real reconfigure round-trip is box-only (lavapipe); this covers
// the branch selection without a GPU.
func TestClassifyAcquire(t *testing.T) {
	statusErr := func(s wgpu.SurfaceGetCurrentTextureStatus) error {
		return &wgpu.SurfaceStatusError{Status: s}
	}
	cases := []struct {
		name string
		err  error
		want acquireAction
	}{
		{"outdated → reconfigure", statusErr(wgpu.SurfaceGetCurrentTextureStatusOutdated), acquireReconfig},
		{"timeout → skip", statusErr(wgpu.SurfaceGetCurrentTextureStatusTimeout), acquireSkip},
		{"lost → fatal", statusErr(wgpu.SurfaceGetCurrentTextureStatusLost), acquireFatal},
		{"device-lost → fatal", statusErr(wgpu.SurfaceGetCurrentTextureStatusDeviceLost), acquireFatal},
		{"out-of-memory → fatal", statusErr(wgpu.SurfaceGetCurrentTextureStatusOutOfMemory), acquireFatal},
		{"unexpected success → skip", statusErr(wgpu.SurfaceGetCurrentTextureStatusSuccess), acquireSkip},
		{"wrapped outdated → reconfigure", fmt.Errorf("acquire: %w", statusErr(wgpu.SurfaceGetCurrentTextureStatusOutdated)), acquireReconfig},
		{"non-status error → skip", errors.New("validation failure"), acquireSkip},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyAcquire(tc.err); got != tc.want {
				t.Fatalf("classifyAcquire(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}
