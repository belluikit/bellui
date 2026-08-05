//go:build !gl

package webgpu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAbandonFrame guards the #283 discard path: the mobile driver calls
// AbandonFrame instead of Present when the native window died while the frame
// was being recorded, and it runs on the same lazily-initialized painter as the
// rest of the mobile paint path — so it must survive a nil device, and it must
// always leave the painter out of the framing state rather than stranding a
// half-recorded frame that the next Clear would mistake for an open one.
func TestAbandonFrame(t *testing.T) {
	// Before Init succeeds (the mobile surface is built from the native window,
	// absent on the first paint) there is no device to discard through.
	nilDev := &painter{framing: true}
	assert.NotPanics(t, nilDev.AbandonFrame)

	// Device up but no frame acquired — BeginFrame failed, or the frame was
	// already discarded by a surface reconfigure. Nothing to discard, but the
	// stale framing flag must still be cleared.
	noFrame := &painter{dev: &Device{}, framing: true}
	assert.NotPanics(t, noFrame.AbandonFrame)
	assert.False(t, noFrame.framing, "framing must be cleared even without an open frame")

	// Not framing at all: a no-op, and it must not report otherwise.
	idle := &painter{dev: &Device{}}
	assert.NotPanics(t, idle.AbandonFrame)
	assert.False(t, idle.framing)
}
