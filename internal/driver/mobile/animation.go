package mobile

import "github.com/belluikit/bellui"

func (d *driver) StartAnimation(a *bellui.Animation) {
	d.animation.Start(a)
}

func (d *driver) StopAnimation(a *bellui.Animation) {
	d.animation.Stop(a)
}
