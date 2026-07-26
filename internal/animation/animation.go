package animation

import (
	"time"

	"github.com/belluikit/bellui"
)

type anim struct {
	a           *bellui.Animation
	end         time.Time
	repeatsLeft int
	reverse     bool
	start       time.Time
	total       int64
	stopped     bool
}

func newAnim(a *bellui.Animation) *anim {
	animate := &anim{a: a, start: time.Now(), end: time.Now().Add(a.Duration)}
	animate.total = animate.end.Sub(animate.start).Milliseconds()
	animate.repeatsLeft = a.RepeatCount
	return animate
}

func (a *anim) setStopped() {
	a.stopped = true
}

func (a *anim) isStopped() bool {
	return a.stopped
}
