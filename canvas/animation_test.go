package canvas

import (
	"image/color"
	"testing"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
	"github.com/stretchr/testify/assert"
)

func TestNewColorAnimation(t *testing.T) {
	start := color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF}
	stop := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}

	var current color.Color
	anim := NewColorRGBAAnimation(start, stop, time.Second, func(c color.Color) {
		current = c
	})

	anim.Tick(0.0)
	assert.Equal(t, color.RGBA{R: 0, G: 0, B: 0, A: 0xFF}, current)
	anim.Tick(0.5)
	assert.Equal(t, color.RGBA{R: 0x7F, G: 0x7F, B: 0x7F, A: 0xFF}, current)
	anim.Tick(1.0)
	assert.Equal(t, color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, current)
}

func TestNewPositionAnimation(t *testing.T) {
	start := geo.NewPos(110, 10)
	stop := geo.NewPos(10, 110)

	var current geo.Position
	anim := NewPositionAnimation(start, stop, time.Second, func(p geo.Position) {
		current = p
	})

	anim.Tick(0.0)
	assert.Equal(t, start, current)
	anim.Tick(0.5)
	assert.Equal(t, geo.NewPos(60, 60), current)
	anim.Tick(1.0)
	assert.Equal(t, stop, current)
}

func TestNewSizeAnimation(t *testing.T) {
	start := geo.NewSize(110, 10)
	stop := geo.NewSize(10, 110)

	var current geo.Size
	anim := NewSizeAnimation(start, stop, time.Second, func(s geo.Size) {
		current = s
	})

	anim.Tick(0.0)
	assert.Equal(t, start, current)
	anim.Tick(0.5)
	assert.Equal(t, geo.NewSize(60, 60), current)
	anim.Tick(1.0)
	assert.Equal(t, stop, current)
}

func TestNewFloat32Animation(t *testing.T) {
	var current float32
	anim := NewFloat32Animation(0, 100, time.Second, func(v float32) {
		current = v
	})

	anim.Tick(0.0)
	assert.Equal(t, float32(0), current)
	anim.Tick(0.5)
	assert.Equal(t, float32(50), current)
	anim.Tick(1.0)
	assert.Equal(t, float32(100), current)
}

func TestNewFloat32Animation_NegativeRange(t *testing.T) {
	var current float32
	anim := NewFloat32Animation(100, -100, time.Second, func(v float32) {
		current = v
	})

	anim.Tick(0.0)
	assert.Equal(t, float32(100), current)
	anim.Tick(0.5)
	assert.Equal(t, float32(0), current)
	anim.Tick(1.0)
	assert.Equal(t, float32(-100), current)
}

func TestNewPathAnimation(t *testing.T) {
	points := []geo.Position{
		geo.NewPos(0, 0),
		geo.NewPos(100, 0),
		geo.NewPos(100, 100),
	}

	var current geo.Position
	anim := NewPathAnimation(points, time.Second, func(p geo.Position) {
		current = p
	})

	anim.Tick(0.0)
	assert.Equal(t, geo.NewPos(0, 0), current)

	// Halfway through first segment
	anim.Tick(0.25)
	assert.Equal(t, geo.NewPos(50, 0), current)

	// At the boundary between segments
	anim.Tick(0.5)
	assert.Equal(t, geo.NewPos(100, 0), current)

	// Halfway through second segment
	anim.Tick(0.75)
	assert.Equal(t, geo.NewPos(100, 50), current)

	// End
	anim.Tick(1.0)
	assert.Equal(t, geo.NewPos(100, 100), current)
}

func TestNewPathAnimation_PanicsOnTooFewPoints(t *testing.T) {
	assert.Panics(t, func() {
		NewPathAnimation([]geo.Position{geo.NewPos(0, 0)}, time.Second, func(geo.Position) {})
	})
}

func TestAnimatedFloat32_InitialValue(t *testing.T) {
	a := NewAnimatedFloat32(42)
	assert.Equal(t, float32(42), a.Value())
}

func TestAnimatedFloat32_ValueTracking(t *testing.T) {
	a := NewAnimatedFloat32(0)

	// Simulate what AnimateTo would set up internally.
	var received float32
	start := a.value
	target := float32(100)
	delta := target - start

	a.anim = &bellui.Animation{
		Duration: time.Second,
		Tick: func(done float32) {
			a.value = scaleVal(start, delta, done)
			received = a.value
		},
	}

	// Simulate runner ticking at 50%
	a.anim.Tick(0.5)
	assert.Equal(t, float32(50), a.Value())
	assert.Equal(t, float32(50), received)

	// Simulate runner ticking at 100%
	a.anim.Tick(1.0)
	assert.Equal(t, float32(100), a.Value())
}

func TestAnimatedFloat32_Interruption(t *testing.T) {
	a := NewAnimatedFloat32(0)

	// First animation: 0 → 100, tick to 50%
	start1 := a.value
	delta1 := float32(100) - start1
	a.anim = &bellui.Animation{
		Duration: time.Second,
		Tick: func(done float32) {
			a.value = scaleVal(start1, delta1, done)
		},
	}
	a.anim.Tick(0.5)
	assert.Equal(t, float32(50), a.Value())

	// Second animation: starts from interrupted value (50) → 200
	start2 := a.value
	delta2 := float32(200) - start2
	a.anim = &bellui.Animation{
		Duration: time.Second,
		Tick: func(done float32) {
			a.value = scaleVal(start2, delta2, done)
		},
	}

	a.anim.Tick(0.0)
	assert.Equal(t, float32(50), a.Value()) // starts from interrupted value

	a.anim.Tick(0.5)
	assert.Equal(t, float32(125), a.Value()) // 50 + (200-50)*0.5

	a.anim.Tick(1.0)
	assert.Equal(t, float32(200), a.Value())
}

func TestAnimatedPosition_InitialValue(t *testing.T) {
	a := NewAnimatedPosition(geo.NewPos(10, 20))
	assert.Equal(t, geo.NewPos(10, 20), a.Value())
}

func TestAnimatedPosition_ValueTracking(t *testing.T) {
	a := NewAnimatedPosition(geo.NewPos(0, 0))

	start := a.value
	target := geo.NewPos(100, 200)
	xDelta := target.X - start.X
	yDelta := target.Y - start.Y

	a.anim = &bellui.Animation{
		Duration: time.Second,
		Tick: func(done float32) {
			a.value = geo.NewPos(scaleVal(start.X, xDelta, done), scaleVal(start.Y, yDelta, done))
		},
	}

	a.anim.Tick(0.5)
	assert.Equal(t, geo.NewPos(50, 100), a.Value())

	a.anim.Tick(1.0)
	assert.Equal(t, geo.NewPos(100, 200), a.Value())
}
