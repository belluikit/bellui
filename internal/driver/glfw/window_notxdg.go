//go:build !linux && !freebsd && !openbsd && !netbsd

package glfw

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
)

func (w *window) platformResize(canvasSize geo.Size) {
	d, ok := bellui.CurrentApp().Driver().(*gLDriver)
	if !ok { // don't wait to redraw in this way if we are running on test
		w.canvas.Resize(canvasSize)
		return
	}

	w.canvas.Resize(canvasSize)
	d.repaintWindow(w)
}
