//go:build !gl

package webgpu

import (
	"errors"
	"fmt"
	"testing"

	"github.com/belluikit/webgpu/wgpu"
)

// TestClassifyAcquire locks the BeginFrame surface-acquire recovery table (#385):
// Outdated → reconfigure+retry, Lost/Error → fatal (log+skip), everything else
// (Timeout, Occluded, an unexpected Success, wrapping, or a non-status error)
// → plain skip. The real reconfigure round-trip is box-only (lavapipe); this covers
// the branch selection without a GPU.
func TestClassifyAcquire(t *testing.T) {
	statusErr := func(s wgpu.SurfaceGetCurrentTextureStatus) error {
		return &SurfaceStatusError{Status: s}
	}
	cases := []struct {
		name string
		err  error
		want acquireAction
	}{
		{"outdated → reconfigure", statusErr(wgpu.SurfaceGetCurrentTextureStatusOutdated), acquireReconfig},
		{"timeout → skip", statusErr(wgpu.SurfaceGetCurrentTextureStatusTimeout), acquireSkip},
		{"occluded → skip", statusErr(wgpu.SurfaceGetCurrentTextureStatusOccluded), acquireSkip},
		{"lost → fatal", statusErr(wgpu.SurfaceGetCurrentTextureStatusLost), acquireFatal},
		{"error → fatal", statusErr(wgpu.SurfaceGetCurrentTextureStatusError), acquireFatal},
		{"unexpected success → skip", statusErr(wgpu.SurfaceGetCurrentTextureStatusSuccessOptimal), acquireSkip},
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
