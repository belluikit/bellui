//go:build !windows

package glfw

import (
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/scale"
)

func (w *window) setDarkMode() {
}

func (w *window) computeCanvasSize(width, height int) geo.Size {
	return geo.NewSize(scale.ToBelluiCoordinate(w.canvas, width), scale.ToBelluiCoordinate(w.canvas, height))
}
