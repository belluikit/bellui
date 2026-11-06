//go:build mobile

package widget

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
)

// hostAbsolutePosition returns a host object's canvas-absolute origin: its
// point (0, 0) through hostPointAbsolute. The handle tests place fingers
// from it on untransformed hosts.
func hostAbsolutePosition(obj core.CanvasObject) geo.Position {
	return hostPointAbsolute(obj, geo.Position{})
}
