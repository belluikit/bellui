package glfw

import "github.com/belluikit/bellui"

func (d *gLDriver) StartAnimation(a *bellui.Animation) {
	d.animation.Start(a)
	// An idle loop sleeps for up to idleTick; wake it so the animation's first
	// frame is not late when it starts from another goroutine.
	wakeMainLoop()
}

func (d *gLDriver) StopAnimation(a *bellui.Animation) {
	d.animation.Stop(a)
}
