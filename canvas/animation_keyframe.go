package canvas

import (
	"time"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
)

// Keyframe defines a value at a specific point in an animation's progress.
// The Curve field specifies the easing from the previous keyframe to this one;
// nil means linear interpolation for that segment.
type Keyframe struct {
	Progress float32               // 0.0 to 1.0
	Value    float32               // value at this keyframe
	Curve    bellui.AnimationCurve // easing from previous keyframe to this one; nil = linear
}

// NewKeyframeAnimation creates an animation that transitions through the given keyframes
// over the specified duration. Each keyframe specifies a value at a progress point,
// with optional per-segment easing curves.
//
// Requirements:
//   - At least 2 keyframes
//   - First keyframe must have Progress == 0.0 (within a small epsilon)
//   - Last keyframe must have Progress == 1.0 (within a small epsilon)
//   - Keyframes must be sorted by Progress (ascending)
//
// Panics if requirements are not met. The first/last Progress checks use an epsilon
// tolerance so a caller computing progress as a float (e.g. 0.9999998) does not panic.
// The returned animation uses AnimationLinear because per-segment curves handle easing.
// You should call Start() on the returned animation.
func NewKeyframeAnimation(keyframes []Keyframe, d time.Duration, fn func(float32)) *bellui.Animation {
	validateKeyframes(keyframes)

	// Copy to avoid mutation.
	kf := make([]Keyframe, len(keyframes))
	copy(kf, keyframes)

	return &bellui.Animation{
		Duration: d,
		Curve:    bellui.AnimationLinear,
		Tick: func(done float32) {
			if done >= 1.0 {
				fn(kf[len(kf)-1].Value)
				return
			}
			if done <= 0 {
				fn(kf[0].Value)
				return
			}

			// Find the active segment (linear scan, typically <10 keyframes).
			idx := 0
			for i := 1; i < len(kf); i++ {
				if kf[i].Progress > done {
					break
				}
				idx = i
			}

			a, b := kf[idx], kf[idx+1]
			span := b.Progress - a.Progress
			if span <= 0 {
				fn(b.Value)
				return
			}

			local := (done - a.Progress) / span
			curve := b.Curve
			if curve != nil {
				local = curve(local)
			}

			fn(scaleVal(a.Value, b.Value-a.Value, local))
		},
	}
}

// PositionKeyframe defines a position at a specific point in an animation's progress.
type PositionKeyframe struct {
	Progress float32
	Value    geo.Position
	Curve    bellui.AnimationCurve // easing from previous keyframe to this one; nil = linear
}

// NewPositionKeyframeAnimation creates an animation that transitions through position
// keyframes over the specified duration. See NewKeyframeAnimation for requirements.
func NewPositionKeyframeAnimation(keyframes []PositionKeyframe, d time.Duration, fn func(geo.Position)) *bellui.Animation {
	validatePositionKeyframes(keyframes)

	kf := make([]PositionKeyframe, len(keyframes))
	copy(kf, keyframes)

	return &bellui.Animation{
		Duration: d,
		Curve:    bellui.AnimationLinear,
		Tick: func(done float32) {
			if done >= 1.0 {
				fn(kf[len(kf)-1].Value)
				return
			}
			if done <= 0 {
				fn(kf[0].Value)
				return
			}

			idx := 0
			for i := 1; i < len(kf); i++ {
				if kf[i].Progress > done {
					break
				}
				idx = i
			}

			a, b := kf[idx], kf[idx+1]
			span := b.Progress - a.Progress
			if span <= 0 {
				fn(b.Value)
				return
			}

			local := (done - a.Progress) / span
			curve := b.Curve
			if curve != nil {
				local = curve(local)
			}

			fn(geo.NewPos(
				scaleVal(a.Value.X, b.Value.X-a.Value.X, local),
				scaleVal(a.Value.Y, b.Value.Y-a.Value.Y, local),
			))
		},
	}
}

func validateKeyframes(kf []Keyframe) {
	validateProgress(len(kf), func(i int) float32 { return kf[i].Progress })
}

func validatePositionKeyframes(kf []PositionKeyframe) {
	validateProgress(len(kf), func(i int) float32 { return kf[i].Progress })
}

// keyframeProgressEpsilon is the tolerance used when checking that the first and
// last keyframe sit exactly at progress 0.0 and 1.0. Float-computed endpoints
// (e.g. 0.9999998) are accepted rather than panicking a public constructor.
const keyframeProgressEpsilon = 1e-4

// validateProgress checks the shared keyframe requirements (at least 2 keyframes,
// endpoints at 0.0/1.0 within keyframeProgressEpsilon, ascending Progress) against
// any keyframe slice via a progress accessor. It panics on genuine misuse.
func validateProgress(n int, progressAt func(int) float32) {
	if n < 2 {
		panic("canvas: keyframe animation requires at least 2 keyframes")
	}
	if abs32(progressAt(0)) > keyframeProgressEpsilon {
		panic("canvas: first keyframe must have Progress == 0.0")
	}
	if abs32(progressAt(n-1)-1.0) > keyframeProgressEpsilon {
		panic("canvas: last keyframe must have Progress == 1.0")
	}
	for i := 1; i < n; i++ {
		if progressAt(i) < progressAt(i-1) {
			panic("canvas: keyframes must be sorted by Progress")
		}
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
