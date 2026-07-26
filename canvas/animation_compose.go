package canvas

import (
	"time"

	"github.com/belluikit/bellui"
)

// NewParallelAnimation creates an animation that runs all children simultaneously.
// The duration is the maximum of the children's durations. Children with shorter
// durations reach their final value and hold it until the overall animation completes.
//
// Each child's own Curve is applied; the returned animation uses AnimationLinear so
// children control their own easing. Children's RepeatCount and AutoReverse fields
// are ignored — set those on the returned animation if needed.
func NewParallelAnimation(children ...*bellui.Animation) *bellui.Animation {
	if len(children) == 0 {
		return &bellui.Animation{Duration: 0, Tick: func(float32) {}}
	}

	var maxDur time.Duration
	for _, c := range children {
		if c.Duration > maxDur {
			maxDur = c.Duration
		}
	}
	maxMs := float32(maxDur.Milliseconds())

	// Precompute scale factors: maxDuration / childDuration.
	scales := make([]float32, len(children))
	for i, c := range children {
		if c.Duration > 0 {
			scales[i] = maxMs / float32(c.Duration.Milliseconds())
		} else {
			scales[i] = 1
		}
	}

	return &bellui.Animation{
		Duration: maxDur,
		Curve:    bellui.AnimationLinear,
		Tick: func(done float32) {
			for i, c := range children {
				childDone := done * scales[i]
				if childDone > 1 {
					childDone = 1
				}

				curve := c.Curve
				if curve == nil {
					curve = bellui.AnimationEaseInOut
				}
				c.Tick(curve(childDone))
			}
		},
	}
}

// NewSequenceAnimation creates an animation that runs children one after another
// in the order given. The total duration is the sum of all children's durations.
//
// Each child's own Curve is applied; the returned animation uses AnimationLinear.
// Children's RepeatCount and AutoReverse fields are ignored.
func NewSequenceAnimation(children ...*bellui.Animation) *bellui.Animation {
	if len(children) == 0 {
		return &bellui.Animation{Duration: 0, Tick: func(float32) {}}
	}

	var totalDur time.Duration
	for _, c := range children {
		totalDur += c.Duration
	}
	totalMs := float32(totalDur.Milliseconds())

	// Precompute cumulative boundaries in [0, 1] progress space.
	boundaries := make([]float32, len(children)+1)
	var cumulative time.Duration
	for i, c := range children {
		boundaries[i] = float32(cumulative.Milliseconds()) / totalMs
		cumulative += c.Duration
	}
	boundaries[len(children)] = 1.0

	lastCompleted := -1

	return &bellui.Animation{
		Duration: totalDur,
		Curve:    bellui.AnimationLinear,
		Tick: func(done float32) {
			// Finalize any children that have been passed.
			for i := lastCompleted + 1; i < len(children); i++ {
				if done < boundaries[i+1] {
					break
				}
				curve := children[i].Curve
				if curve == nil {
					curve = bellui.AnimationEaseInOut
				}
				children[i].Tick(curve(1.0))
				lastCompleted = i
			}

			// Tick the currently active child.
			active := lastCompleted + 1
			if active >= len(children) {
				return
			}

			start := boundaries[active]
			end := boundaries[active+1]
			span := end - start
			if span <= 0 {
				return
			}

			localDone := (done - start) / span
			if localDone < 0 {
				localDone = 0
			}
			if localDone > 1 {
				localDone = 1
			}

			curve := children[active].Curve
			if curve == nil {
				curve = bellui.AnimationEaseInOut
			}
			children[active].Tick(curve(localDone))
		},
	}
}

// StaggeredEntry pairs an animation with a start delay for use with NewStaggeredAnimation.
type StaggeredEntry struct {
	Animation *bellui.Animation
	Delay     time.Duration
}

// NewStaggeredAnimation creates an animation that runs children with staggered start offsets.
// Each entry specifies an animation and its delay from the start of the overall animation.
// The total duration is max(entry.Delay + entry.Animation.Duration) across all entries.
//
// Each child's own Curve is applied; the returned animation uses AnimationLinear.
// Children's RepeatCount and AutoReverse fields are ignored.
func NewStaggeredAnimation(entries []StaggeredEntry) *bellui.Animation {
	if len(entries) == 0 {
		return &bellui.Animation{Duration: 0, Tick: func(float32) {}}
	}

	var maxDur time.Duration
	for _, e := range entries {
		end := e.Delay + e.Animation.Duration
		if end > maxDur {
			maxDur = end
		}
	}
	totalMs := float32(maxDur.Milliseconds())

	// Precompute each entry's window in [0, 1] progress space.
	type window struct {
		start float32 // entry.Delay / totalDur
		end   float32 // (entry.Delay + entry.Duration) / totalDur
	}
	windows := make([]window, len(entries))
	for i, e := range entries {
		windows[i] = window{
			start: float32(e.Delay.Milliseconds()) / totalMs,
			end:   float32((e.Delay + e.Animation.Duration).Milliseconds()) / totalMs,
		}
	}

	return &bellui.Animation{
		Duration: maxDur,
		Curve:    bellui.AnimationLinear,
		Tick: func(done float32) {
			for i, e := range entries {
				w := windows[i]
				if done < w.start {
					continue // not started yet
				}

				span := w.end - w.start
				var childDone float32
				if span <= 0 || done >= w.end {
					childDone = 1
				} else {
					childDone = (done - w.start) / span
				}

				curve := e.Animation.Curve
				if curve == nil {
					curve = bellui.AnimationEaseInOut
				}
				e.Animation.Tick(curve(childDone))
			}
		},
	}
}
