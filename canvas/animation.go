package canvas

import (
	"image/color"
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
)

const (
	// DurationStandard is the time a standard interface animation will run.
	//
	// Since: 2.0
	DurationStandard = time.Millisecond * 300
	// DurationShort is the time a subtle or small transition should use.
	//
	// Since: 2.0
	DurationShort = time.Millisecond * 150

	// MD3 duration tokens.
	DurationShort1  = time.Millisecond * 50
	DurationShort2  = time.Millisecond * 100
	DurationShort3  = DurationShort // 150ms
	DurationShort4  = time.Millisecond * 200
	DurationMedium1 = time.Millisecond * 250
	DurationMedium2 = DurationStandard // 300ms
	DurationMedium3 = time.Millisecond * 350
	DurationMedium4 = time.Millisecond * 400
	DurationLong1   = time.Millisecond * 450
	DurationLong2   = time.Millisecond * 500
)

// NewColorRGBAAnimation sets up a new animation that will transition from the start to stop Color over
// the specified Duration. The colour transition will move linearly through the RGB colour space.
// The content of fn should apply the color values to an object and refresh it.
// You should call Start() on the returned animation to start it.
//
// Since: 2.0
func NewColorRGBAAnimation(start, stop color.Color, d time.Duration, fn func(color.Color)) *bellui.Animation {
	r1, g1, b1, a1 := start.RGBA()
	r2, g2, b2, a2 := stop.RGBA()

	rStart := int(r1 >> 8)
	gStart := int(g1 >> 8)
	bStart := int(b1 >> 8)
	aStart := int(a1 >> 8)
	rDelta := float32(int(r2>>8) - rStart)
	gDelta := float32(int(g2>>8) - gStart)
	bDelta := float32(int(b2>>8) - bStart)
	aDelta := float32(int(a2>>8) - aStart)

	return &bellui.Animation{
		Duration: d,
		Tick: func(done float32) {
			fn(color.RGBA{
				R: scaleChannel(rStart, rDelta, done), G: scaleChannel(gStart, gDelta, done),
				B: scaleChannel(bStart, bDelta, done), A: scaleChannel(aStart, aDelta, done),
			})
		},
	}
}

// NewPositionAnimation sets up a new animation that will transition from the start to stop Position over
// the specified Duration. The content of fn should apply the position value to an object for the change
// to be visible. You should call Start() on the returned animation to start it.
//
// Since: 2.0
func NewPositionAnimation(start, stop geo.Position, d time.Duration, fn func(geo.Position)) *bellui.Animation {
	xDelta := stop.X - start.X
	yDelta := stop.Y - start.Y

	return &bellui.Animation{
		Duration: d,
		Tick: func(done float32) {
			fn(geo.NewPos(scaleVal(start.X, xDelta, done), scaleVal(start.Y, yDelta, done)))
		},
	}
}

// NewSizeAnimation sets up a new animation that will transition from the start to stop Size over
// the specified Duration. The content of fn should apply the size value to an object for the change
// to be visible. You should call Start() on the returned animation to start it.
//
// Since: 2.0
func NewSizeAnimation(start, stop geo.Size, d time.Duration, fn func(geo.Size)) *bellui.Animation {
	widthDelta := stop.Width - start.Width
	heightDelta := stop.Height - start.Height

	return &bellui.Animation{
		Duration: d,
		Tick: func(done float32) {
			fn(geo.NewSize(scaleVal(start.Width, widthDelta, done), scaleVal(start.Height, heightDelta, done)))
		},
	}
}

// NewFloat32Animation sets up a new animation that will transition from start to stop over
// the specified Duration. This is the general-purpose scalar interpolation, suitable for
// animating opacity, rotation angles, scale factors, or any single float32 property.
// The content of fn should apply the value to an object and refresh it.
// You should call Start() on the returned animation to start it.
func NewFloat32Animation(start, stop float32, d time.Duration, fn func(float32)) *bellui.Animation {
	delta := stop - start

	return &bellui.Animation{
		Duration: d,
		Tick: func(done float32) {
			fn(scaleVal(start, delta, done))
		},
	}
}

// NewPathAnimation sets up a new animation that will move through the given waypoints over
// the specified Duration. The points are traversed in order with equal time per segment.
// At least 2 points are required. The content of fn should apply the position to an object
// and refresh it. You should call Start() on the returned animation to start it.
func NewPathAnimation(points []geo.Position, d time.Duration, fn func(geo.Position)) *bellui.Animation {
	if len(points) < 2 {
		panic("canvas: NewPathAnimation requires at least 2 points")
	}

	segments := float32(len(points) - 1)

	return &bellui.Animation{
		Duration: d,
		Tick: func(done float32) {
			if done >= 1.0 {
				fn(points[len(points)-1])
				return
			}

			scaled := done * segments
			idx := int(scaled)
			if idx >= len(points)-1 {
				idx = len(points) - 2
			}
			local := scaled - float32(idx)

			a, b := points[idx], points[idx+1]
			fn(geo.NewPos(
				scaleVal(a.X, b.X-a.X, local),
				scaleVal(a.Y, b.Y-a.Y, local),
			))
		},
	}
}

// AnimatedFloat32 tracks a float32 value that can be smoothly animated to new targets.
// When AnimateTo is called while an animation is running, the current (interrupted)
// value is captured as the start of the new animation, enabling seamless transitions.
//
// AnimatedFloat32 must only be used from the main thread (the same goroutine
// that processes UI events and animation ticks).
type AnimatedFloat32 struct {
	value float32
	anim  *bellui.Animation
}

// NewAnimatedFloat32 creates an AnimatedFloat32 with the given initial value.
func NewAnimatedFloat32(initial float32) *AnimatedFloat32 {
	return &AnimatedFloat32{value: initial}
}

// AnimateTo starts a new animation from the current value to target. If an animation
// is already running, it is stopped and the new animation starts from the interrupted value.
// The fn callback is called each frame with the current value; it should apply the value
// to a canvas object and refresh it.
func (a *AnimatedFloat32) AnimateTo(target float32, d time.Duration, curve bellui.AnimationCurve, fn func(float32)) {
	if a.anim != nil {
		a.anim.Stop()
	}

	start := a.value
	delta := target - start

	a.anim = &bellui.Animation{
		Duration: d,
		Curve:    curve,
		Tick: func(done float32) {
			a.value = scaleVal(start, delta, done)
			fn(a.value)
		},
	}
	a.anim.Start()
}

// Value returns the current animated value.
func (a *AnimatedFloat32) Value() float32 {
	return a.value
}

// Stop stops the current animation if one is running. The value remains at its
// current (interrupted) position.
func (a *AnimatedFloat32) Stop() {
	if a.anim != nil {
		a.anim.Stop()
		a.anim = nil
	}
}

// AnimatedPosition tracks a position that can be smoothly animated to new targets.
// When AnimateTo is called while an animation is running, the current (interrupted)
// position is captured as the start of the new animation.
//
// AnimatedPosition must only be used from the main thread.
type AnimatedPosition struct {
	value geo.Position
	anim  *bellui.Animation
}

// NewAnimatedPosition creates an AnimatedPosition with the given initial position.
func NewAnimatedPosition(initial geo.Position) *AnimatedPosition {
	return &AnimatedPosition{value: initial}
}

// AnimateTo starts a new animation from the current position to target. If an animation
// is already running, it is stopped and the new animation starts from the interrupted position.
func (a *AnimatedPosition) AnimateTo(target geo.Position, d time.Duration, curve bellui.AnimationCurve, fn func(geo.Position)) {
	if a.anim != nil {
		a.anim.Stop()
	}

	start := a.value
	xDelta := target.X - start.X
	yDelta := target.Y - start.Y

	a.anim = &bellui.Animation{
		Duration: d,
		Curve:    curve,
		Tick: func(done float32) {
			a.value = geo.NewPos(scaleVal(start.X, xDelta, done), scaleVal(start.Y, yDelta, done))
			fn(a.value)
		},
	}
	a.anim.Start()
}

// Value returns the current animated position.
func (a *AnimatedPosition) Value() geo.Position {
	return a.value
}

// Stop stops the current animation if one is running. The position remains at its
// current (interrupted) value.
func (a *AnimatedPosition) Stop() {
	if a.anim != nil {
		a.anim.Stop()
		a.anim = nil
	}
}

func scaleChannel(start int, diff, done float32) uint8 {
	return uint8(start + int(diff*done))
}

func scaleVal(start float32, delta, done float32) float32 {
	return start + delta*done
}
