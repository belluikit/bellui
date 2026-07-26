// Package animate provides chart animation controllers for entrance and data transitions.
package animate

import (
	"time"

	"github.com/belluikit/bellui"
)

// Controller manages animations for a chart widget.
type Controller struct {
	refreshFunc func() // called on each animation tick to trigger re-render
	current     *bellui.Animation
	Progress    float32 // 0.0 to 1.0, read by series renderers
	Active      bool    // true while an animation is running
}

// NewController creates an animation controller that calls refreshFunc on each tick.
func NewController(refreshFunc func()) *Controller {
	return &Controller{refreshFunc: refreshFunc, Progress: 1.0}
}

// Enter runs an entrance animation over the given duration.
// Series renderers should read Controller.Progress to interpolate from 0 (hidden) to 1 (final).
func (c *Controller) Enter(duration time.Duration) {
	c.Stop()
	c.Progress = 0
	c.Active = true

	c.current = &bellui.Animation{
		Duration: duration,
		Curve:    bellui.AnimationEaseStandardDecelerate,
		Tick: func(p float32) {
			c.Progress = p
			if c.refreshFunc != nil {
				c.refreshFunc()
			}
		},
	}
	c.current.Start()
}

// Transition runs an animation from the current progress to 1.0.
// Useful for data updates where old positions morph to new positions.
func (c *Controller) Transition(duration time.Duration) {
	c.Stop()
	startProgress := c.Progress
	c.Active = true

	c.current = &bellui.Animation{
		Duration: duration,
		Curve:    bellui.AnimationEaseStandard,
		Tick: func(p float32) {
			c.Progress = startProgress + (1.0-startProgress)*p
			if c.refreshFunc != nil {
				c.refreshFunc()
			}
		},
	}
	c.current.Start()
}

// Stop cancels any running animation and sets progress to 1.0.
func (c *Controller) Stop() {
	if c.current != nil {
		c.current.Stop()
		c.current = nil
	}
	c.Progress = 1.0
	c.Active = false
}

// DefaultEntranceDuration is the default duration for entrance animations.
const DefaultEntranceDuration = 400 * time.Millisecond

// DefaultTransitionDuration is the default duration for data transition animations.
const DefaultTransitionDuration = 300 * time.Millisecond
