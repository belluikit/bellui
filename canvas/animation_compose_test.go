package canvas

import (
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/stretchr/testify/assert"
)

func TestParallelAnimation(t *testing.T) {
	var v1, v2 float32
	c1 := NewFloat32Animation(0, 100, time.Second, func(v float32) { v1 = v })
	c2 := NewFloat32Animation(0, 200, 500*time.Millisecond, func(v float32) { v2 = v })

	anim := NewParallelAnimation(c1, c2)
	assert.Equal(t, time.Second, anim.Duration)

	// At 0: both at start
	anim.Tick(0.0)
	assert.Equal(t, float32(0), v1)
	assert.Equal(t, float32(0), v2)

	// At 0.5: c1 halfway, c2 complete (its duration is half)
	anim.Tick(0.5)
	assert.Equal(t, float32(50), v1)
	assert.Equal(t, float32(200), v2)

	// At 1.0: both complete
	anim.Tick(1.0)
	assert.Equal(t, float32(100), v1)
	assert.Equal(t, float32(200), v2)
}

func TestParallelAnimation_Empty(t *testing.T) {
	anim := NewParallelAnimation()
	assert.Equal(t, time.Duration(0), anim.Duration)
	anim.Tick(1.0) // should not panic
}

func TestSequenceAnimation(t *testing.T) {
	var v1, v2, v3 float32
	c1 := NewFloat32Animation(0, 10, time.Second, func(v float32) { v1 = v })
	c1.Curve = bellui.AnimationLinear
	c2 := NewFloat32Animation(0, 20, time.Second, func(v float32) { v2 = v })
	c2.Curve = bellui.AnimationLinear
	c3 := NewFloat32Animation(0, 30, time.Second, func(v float32) { v3 = v })
	c3.Curve = bellui.AnimationLinear

	anim := NewSequenceAnimation(c1, c2, c3)
	assert.Equal(t, 3*time.Second, anim.Duration)

	// At start
	anim.Tick(0.0)
	assert.Equal(t, float32(0), v1)

	// During first child (1/6 = midway through first child)
	anim.Tick(1.0 / 6.0)
	assert.InDelta(t, float32(5), v1, 0.1)
	assert.Equal(t, float32(0), v2)

	// At first child boundary
	anim.Tick(1.0 / 3.0)
	assert.InDelta(t, float32(10), v1, 0.1)

	// During second child
	anim.Tick(0.5)
	assert.InDelta(t, float32(10), v2, 0.1)

	// Complete
	anim.Tick(1.0)
	assert.InDelta(t, float32(10), v1, 0.1)
	assert.InDelta(t, float32(20), v2, 0.1)
	assert.InDelta(t, float32(30), v3, 0.1)
}

func TestSequenceAnimation_TestDriverPattern(t *testing.T) {
	// Simulates the test driver calling Tick(1.0) directly.
	var completed [3]bool
	c1 := &bellui.Animation{Duration: time.Second, Tick: func(d float32) {
		if d >= 1.0 {
			completed[0] = true
		}
	}}
	c2 := &bellui.Animation{Duration: time.Second, Tick: func(d float32) {
		if d >= 1.0 {
			completed[1] = true
		}
	}}
	c3 := &bellui.Animation{Duration: time.Second, Tick: func(d float32) {
		if d >= 1.0 {
			completed[2] = true
		}
	}}

	anim := NewSequenceAnimation(c1, c2, c3)
	anim.Tick(1.0)

	assert.True(t, completed[0])
	assert.True(t, completed[1])
	assert.True(t, completed[2])
}

func TestSequenceAnimation_Empty(t *testing.T) {
	anim := NewSequenceAnimation()
	assert.Equal(t, time.Duration(0), anim.Duration)
	anim.Tick(1.0) // should not panic
}

func TestStaggeredAnimation(t *testing.T) {
	var v1, v2 float32
	c1 := NewFloat32Animation(0, 100, 500*time.Millisecond, func(v float32) { v1 = v })
	c1.Curve = bellui.AnimationLinear
	c2 := NewFloat32Animation(0, 200, 500*time.Millisecond, func(v float32) { v2 = v })
	c2.Curve = bellui.AnimationLinear

	entries := []StaggeredEntry{
		{Animation: c1, Delay: 0},
		{Animation: c2, Delay: 250 * time.Millisecond},
	}

	anim := NewStaggeredAnimation(entries)
	// Total duration = max(0+500, 250+500) = 750ms
	assert.Equal(t, 750*time.Millisecond, anim.Duration)

	// At start: c1 starts, c2 hasn't started
	anim.Tick(0.0)
	assert.Equal(t, float32(0), v1)

	// At 250ms/750ms = 1/3: c1 at 250/500 = 50%, c2 at 0
	anim.Tick(1.0 / 3.0)
	assert.InDelta(t, float32(50), v1, 1)
	assert.Equal(t, float32(0), v2)

	// At 500ms/750ms = 2/3: c1 done, c2 at 250/500 = 50%
	anim.Tick(2.0 / 3.0)
	assert.InDelta(t, float32(100), v1, 1)
	assert.InDelta(t, float32(100), v2, 1)

	// Complete
	anim.Tick(1.0)
	assert.InDelta(t, float32(100), v1, 1)
	assert.InDelta(t, float32(200), v2, 1)
}

func TestStaggeredAnimation_Empty(t *testing.T) {
	anim := NewStaggeredAnimation(nil)
	assert.Equal(t, time.Duration(0), anim.Duration)
	anim.Tick(1.0) // should not panic
}
