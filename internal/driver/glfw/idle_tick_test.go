//go:build !wasm

package glfw

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNextTickDue pins #470: an idle app ticks idleTick after its last tick,
// not on the frame grid, and returns to the grid as soon as it is not idle.
func TestNextTickDue(t *testing.T) {
	const frame = time.Second / 60
	last := time.Unix(1000, 0)
	grid := last.Add(frame)

	assert.Equal(t, grid, nextTickDue(grid, last, false), "busy: the frame grid's next slot")
	assert.Equal(t, last.Add(idleTick), nextTickDue(grid, last, true), "idle: idleTick after the last tick")
	assert.Greater(t, idleTick, 30*frame, "idle ticks should be far rarer than frames")
	assert.LessOrEqual(t, idleTick, time.Second, "housekeeping and window events must not wait longer")
}
