package canvas

import (
	"math"
	"time"

	"github.com/belluikit/bellui"
)

// SpringConfig configures the physics of a spring animation.
// Default values are applied for zero fields: Stiffness=170, Damping=26, Mass=1, MaxDuration=2s.
type SpringConfig struct {
	Stiffness   float32       // spring constant (default 170)
	Damping     float32       // damping coefficient (default 26)
	Mass        float32       // mass (default 1)
	MaxDuration time.Duration // safety duration cap (default 2s)
}

// Preset spring configurations.
var (
	// SpringGentle is a soft, slow spring with moderate oscillation.
	SpringGentle = SpringConfig{Stiffness: 120, Damping: 14, Mass: 1, MaxDuration: 2 * time.Second}

	// SpringBouncy is a lively spring with visible oscillation before settling.
	SpringBouncy = SpringConfig{Stiffness: 300, Damping: 10, Mass: 1, MaxDuration: 2 * time.Second}

	// SpringStiff is a fast, snappy spring with minimal oscillation.
	SpringStiff = SpringConfig{Stiffness: 400, Damping: 28, Mass: 1, MaxDuration: time.Second}
)

func (c SpringConfig) withDefaults() SpringConfig {
	if c.Stiffness == 0 {
		c.Stiffness = 170
	}
	if c.Damping == 0 {
		c.Damping = 26
	}
	if c.Mass == 0 {
		c.Mass = 1
	}
	if c.MaxDuration == 0 {
		c.MaxDuration = 2 * time.Second
	}
	return c
}

// NewSpringAnimation creates a physics-based spring animation. The callback fn receives
// values from 0.0 (start) to 1.0 (settled), following spring dynamics that may overshoot
// and oscillate depending on the configuration.
//
// The animation runs for at most config.MaxDuration. The spring typically settles well
// before that. The returned animation uses AnimationLinear because the spring equation
// provides its own easing.
//
// You should call Start() on the returned animation to start it.
func NewSpringAnimation(config SpringConfig, fn func(float32)) *bellui.Animation {
	config = config.withDefaults()

	w0 := float64(math.Sqrt(float64(config.Stiffness / config.Mass)))
	zeta := float64(config.Damping) / (2 * float64(config.Mass) * w0)
	maxT := config.MaxDuration.Seconds()

	var springFn func(t float64) float64

	if zeta < 1 {
		// Underdamped: oscillates before settling
		wd := w0 * math.Sqrt(1-zeta*zeta)
		springFn = func(t float64) float64 {
			return math.Exp(-zeta*w0*t) * (math.Cos(wd*t) + (zeta*w0/wd)*math.Sin(wd*t))
		}
	} else if zeta == 1 {
		// Critically damped: fastest non-oscillatory return
		springFn = func(t float64) float64 {
			return (1 + w0*t) * math.Exp(-w0*t)
		}
	} else {
		// Overdamped: slow, no oscillation
		s1 := -w0 * (zeta + math.Sqrt(zeta*zeta-1))
		s2 := -w0 * (zeta - math.Sqrt(zeta*zeta-1))
		// Coefficients from initial conditions: x(0)=1, x'(0)=0
		c2 := -s1 / (s2 - s1)
		c1 := 1 - c2
		springFn = func(t float64) float64 {
			return c1*math.Exp(s1*t) + c2*math.Exp(s2*t)
		}
	}

	return &bellui.Animation{
		Duration: config.MaxDuration,
		Curve:    bellui.AnimationLinear,
		Tick: func(done float32) {
			t := float64(done) * maxT
			x := springFn(t)

			// Settlement check: snap when displacement is negligible
			if math.Abs(x) < 0.001 {
				fn(1.0)
				return
			}

			fn(float32(1.0 - x))
		},
	}
}

// NewSpringFloat32Animation creates a spring animation that transitions from start to stop
// with spring dynamics. This is a convenience wrapper around NewSpringAnimation.
//
// You should call Start() on the returned animation to start it.
func NewSpringFloat32Animation(start, stop float32, config SpringConfig, fn func(float32)) *bellui.Animation {
	delta := stop - start
	return NewSpringAnimation(config, func(v float32) {
		fn(start + delta*v)
	})
}
