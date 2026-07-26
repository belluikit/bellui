package glfw

import "github.com/belluikit/bellui"

func (d *gLDriver) StartAnimation(a *bellui.Animation) {
	d.animation.Start(a)
}

func (d *gLDriver) StopAnimation(a *bellui.Animation) {
	d.animation.Stop(a)
}
