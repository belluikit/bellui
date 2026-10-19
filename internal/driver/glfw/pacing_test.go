package glfw

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNextFrameAfter(t *testing.T) {
	const frame = time.Second / 60
	due := time.Unix(1000, 0)

	assert.Equal(t, due.Add(frame), nextFrameAfter(due, due, frame), "on time")
	assert.Equal(t, due.Add(frame), nextFrameAfter(due, due.Add(700*time.Microsecond), frame),
		"a late wake-up keeps the next frame on the grid")
	assert.Equal(t, due.Add(3*frame/2+frame), nextFrameAfter(due, due.Add(3*frame/2), frame),
		"after a stall the grid restarts from now")

	// Sixty frames each woken 0.7 ms late still take sixty frame periods.
	next := due
	for range 60 {
		next = nextFrameAfter(next, next.Add(700*time.Microsecond), frame)
	}
	assert.Equal(t, due.Add(60*frame), next)
}
