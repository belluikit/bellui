//go:build (gl || (!darwin && !linux && !windows && !wasm)) && !ci

package gl

import "testing"

// TestShaderInitFailure_Classification verifies that a failed shader build is
// recorded so the paint loop can degrade gracefully (pre-rebrand tracker #300): any failure trips
// InitFailed(), and only a failure of the base textured-quad ("simple") program
// — which drives the diagnostic message itself — sets baseProgramFailed.
func TestShaderInitFailure_Classification(t *testing.T) {
	t.Run("advanced shader failure keeps base program usable", func(t *testing.T) {
		p := &painter{}
		p.shaderInitFailure("fragment shader compile failed", "ripple", nil)

		if !p.InitFailed() {
			t.Fatal("InitFailed() should be true after any shader failure")
		}
		if p.baseProgramFailed {
			t.Fatal("a ripple failure must not mark the base program as failed")
		}
	})

	t.Run("base program failure is recorded", func(t *testing.T) {
		p := &painter{}
		p.shaderInitFailure("shader not found", "simple", nil)

		if !p.InitFailed() {
			t.Fatal("InitFailed() should be true after a base program failure")
		}
		if !p.baseProgramFailed {
			t.Fatal("a simple failure must mark the base program as failed")
		}
	})

	t.Run("no failure leaves the painter clean", func(t *testing.T) {
		p := &painter{}
		if p.InitFailed() {
			t.Fatal("InitFailed() should be false on a fresh painter")
		}
	})
}
