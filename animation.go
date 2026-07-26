package bellui

import "time"

// AnimationCurve represents an animation algorithm for calculating the progress through a timeline.
// Custom animations can be provided by implementing the "func(float32) float32" definition.
// The input parameter will start at 0.0 when an animation starts and travel up to 1.0 at which point it will end.
// A linear animation would return the same output value as is passed in.
type AnimationCurve func(float32) float32

// AnimationRepeatForever is an AnimationCount value that indicates it should not stop looping.
//
// Since: 2.0
const AnimationRepeatForever = -1

var (
	// AnimationEaseInOut is the default easing, it starts slowly, accelerates to the middle and slows to the end.
	//
	// Since: 2.0
	AnimationEaseInOut = animationEaseInOut
	// AnimationEaseIn starts slowly and accelerates to the end.
	//
	// Since: 2.0
	AnimationEaseIn = animationEaseIn
	// AnimationEaseOut starts at speed and slows to the end.
	//
	// Since: 2.0
	AnimationEaseOut = animationEaseOut
	// AnimationLinear is a linear mapping for animations that progress uniformly through their duration.
	//
	// Since: 2.0
	AnimationLinear = animationLinear

	// AnimationEaseEmphasized is the MD3 emphasized easing — a two-phase cubic bezier
	// that accelerates then decelerates with different curves.
	AnimationEaseEmphasized AnimationCurve = animationEaseEmphasized

	// AnimationEaseStandard is the MD3 standard easing curve.
	AnimationEaseStandard AnimationCurve = animationEaseStandard

	// AnimationEaseStandardAccelerate is the MD3 accelerating curve for exit animations.
	AnimationEaseStandardAccelerate AnimationCurve = animationEaseStandardAccelerate

	// AnimationEaseStandardDecelerate is the MD3 decelerating curve for entrance animations.
	AnimationEaseStandardDecelerate AnimationCurve = animationEaseStandardDecelerate
)

// Animation represents an animated element within a Bellui canvas.
// These animations may control individual objects or entire scenes.
//
// Since: 2.0
type Animation struct {
	AutoReverse bool
	Curve       AnimationCurve
	Duration    time.Duration
	RepeatCount int
	Tick        func(float32)
}

// NewAnimation creates a very basic animation where the callback function will be called for every
// rendered frame between [time.Now] and the specified duration. The callback values start at 0.0 and
// will be 1.0 when the animation completes.
//
// Since: 2.0
func NewAnimation(d time.Duration, fn func(float32)) *Animation {
	return &Animation{Duration: d, Tick: fn}
}

// Start registers the animation with the application run-loop and starts its execution.
func (a *Animation) Start() {
	CurrentApp().Driver().StartAnimation(a)
}

// Stop will end this animation and remove it from the run-loop.
func (a *Animation) Stop() {
	CurrentApp().Driver().StopAnimation(a)
}

func animationEaseIn(val float32) float32 {
	return val * val
}

func animationEaseInOut(val float32) float32 {
	if val <= 0.5 {
		return val * val * 2
	}

	return -1 + (4-val*2)*val
}

func animationEaseOut(val float32) float32 {
	return val * (2 - val)
}

func animationLinear(val float32) float32 {
	return val
}

// --- MD3 easing curves ---

func cubicBezierAt(x1, y1, x2, y2, t float32) float32 {
	// Binary search for the bezier parameter that gives us the right x
	low, high := float32(0), float32(1)
	for i := 0; i < 20; i++ {
		mid := (low + high) / 2
		x := bezierComponent(x1, x2, mid)
		if x < t {
			low = mid
		} else {
			high = mid
		}
	}
	param := (low + high) / 2
	return bezierComponent(y1, y2, param)
}

func bezierComponent(p1, p2, t float32) float32 {
	// B(t) = 3(1-t)²t·p1 + 3(1-t)t²·p2 + t³
	mt := 1 - t
	return 3*mt*mt*t*p1 + 3*mt*t*t*p2 + t*t*t
}

// MD3 emphasized: two-phase — accelerate then decelerate
func animationEaseEmphasized(val float32) float32 {
	if val < 0.5 {
		// Accelerate phase: cubic-bezier(0.3, 0, 0.8, 0.15) mapped to [0, 0.5]
		return cubicBezierAt(0.3, 0, 0.8, 0.15, val*2) * 0.5
	}
	// Decelerate phase: cubic-bezier(0.05, 0.7, 0.1, 1.0) mapped to [0.5, 1.0]
	return 0.5 + cubicBezierAt(0.05, 0.7, 0.1, 1.0, (val-0.5)*2)*0.5
}

// MD3 standard: cubic-bezier(0.2, 0, 0, 1.0)
func animationEaseStandard(val float32) float32 {
	return cubicBezierAt(0.2, 0, 0, 1.0, val)
}

// MD3 standard accelerate: cubic-bezier(0.3, 0, 1, 1)
func animationEaseStandardAccelerate(val float32) float32 {
	return cubicBezierAt(0.3, 0, 1, 1, val)
}

// MD3 standard decelerate: cubic-bezier(0, 0, 0, 1)
func animationEaseStandardDecelerate(val float32) float32 {
	return cubicBezierAt(0, 0, 0, 1, val)
}
