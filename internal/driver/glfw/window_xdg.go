//go:build linux || freebsd || openbsd || netbsd

package glfw

import "github.com/belluikit/bellui/geo"

func (w *window) platformResize(canvasSize geo.Size) {
	w.canvas.Resize(canvasSize)
}
