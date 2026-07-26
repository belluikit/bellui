package canvas

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSpringAnimation_Underdamped(t *testing.T) {
	var values []float32
	anim := NewSpringAnimation(SpringBouncy, func(v float32) {
		values = append(values, v)
	})

	assert.Equal(t, 2*time.Second, anim.Duration)

	// Should start near 0
	anim.Tick(0.0)
	assert.InDelta(t, 0.0, values[0], 0.01)

	// Underdamped springs overshoot: at some point value > 1.0
	values = nil
	overshot := false
	for p := float32(0); p <= 1.0; p += 0.01 {
		anim.Tick(p)
		if len(values) > 0 && values[len(values)-1] > 1.0 {
			overshot = true
		}
	}
	assert.True(t, overshot, "bouncy spring should overshoot 1.0")

	// Should settle at 1.0
	values = nil
	anim.Tick(1.0)
	assert.InDelta(t, 1.0, values[0], 0.01)
}

func TestSpringAnimation_CriticallyDamped(t *testing.T) {
	// Create a critically damped spring: zeta = damping / (2 * mass * w0) = 1
	// w0 = sqrt(stiffness/mass). For stiffness=100, mass=1: w0=10. damping=2*1*10=20.
	config := SpringConfig{Stiffness: 100, Damping: 20, Mass: 1, MaxDuration: 2 * time.Second}

	var values []float32
	anim := NewSpringAnimation(config, func(v float32) {
		values = append(values, v)
	})

	// Critically damped should never overshoot
	overshot := false
	for p := float32(0); p <= 1.0; p += 0.01 {
		anim.Tick(p)
		if len(values) > 0 && values[len(values)-1] > 1.001 {
			overshot = true
		}
	}
	assert.False(t, overshot, "critically damped spring should not overshoot")

	// Should settle at 1.0
	values = nil
	anim.Tick(1.0)
	assert.InDelta(t, 1.0, values[0], 0.01)
}

func TestSpringAnimation_Overdamped(t *testing.T) {
	// Overdamped: high damping relative to stiffness
	config := SpringConfig{Stiffness: 100, Damping: 40, Mass: 1, MaxDuration: 2 * time.Second}

	var values []float32
	anim := NewSpringAnimation(config, func(v float32) {
		values = append(values, v)
	})

	// Should not overshoot
	overshot := false
	prev := float32(0)
	for p := float32(0.01); p <= 1.0; p += 0.01 {
		values = nil
		anim.Tick(p)
		v := values[0]
		if v > 1.001 {
			overshot = true
		}
		// Monotonically increasing (no oscillation)
		assert.GreaterOrEqual(t, v, prev-0.001, "overdamped should be monotonic at p=%f", p)
		prev = v
	}
	assert.False(t, overshot, "overdamped spring should not overshoot")
}

func TestSpringFloat32Animation(t *testing.T) {
	var current float32
	anim := NewSpringFloat32Animation(10, 110, SpringStiff, func(v float32) {
		current = v
	})

	// Start near 10
	anim.Tick(0.0)
	assert.InDelta(t, 10, current, 1)

	// End near 110
	anim.Tick(1.0)
	assert.InDelta(t, 110, current, 1)
}

func TestSpringConfig_Defaults(t *testing.T) {
	config := SpringConfig{}
	config = config.withDefaults()

	assert.Equal(t, float32(170), config.Stiffness)
	assert.Equal(t, float32(26), config.Damping)
	assert.Equal(t, float32(1), config.Mass)
	assert.Equal(t, 2*time.Second, config.MaxDuration)
}
