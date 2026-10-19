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

// TestFrameForRates pins #480: frames follow the fastest monitor's refresh
// rate, 60 Hz when none is known, within 30–240 Hz.
func TestFrameForRates(t *testing.T) {
	assert.Equal(t, time.Second/60, frameForRates(nil), "no monitor known")
	assert.Equal(t, time.Second/60, frameForRates([]int{0}), "a monitor that reports 0 Hz")
	assert.Equal(t, time.Second/144, frameForRates([]int{60, 144}), "the fastest monitor")
	assert.Equal(t, time.Second/120, frameForRates([]int{120}))
	assert.Equal(t, time.Second/240, frameForRates([]int{1000}), "kept to 240 Hz")
	assert.Equal(t, time.Second/30, frameForRates([]int{24}), "kept to 30 Hz")

	// The pacing helpers work with any frame duration, not only 1/60 s.
	frame := frameForRates([]int{144})
	last := time.Unix(1000, 0)
	assert.Equal(t, last.Add(frame), nextFrameAfter(last, last.Add(time.Millisecond), frame))
	assert.Equal(t, last.Add(frame), nextTickDue(last.Add(frame), last, false))
}
