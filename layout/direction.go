package layout

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/lang"
)

// layoutRTL reports whether containers should be mirrored for a right-to-left
// user interface. It is a thin wrapper over [lang.IsRTLDirection] so the
// standard layouts can gate their mirroring on a single, locale-aware switch.
// When false (the default, left-to-right) every layout keeps its original
// behaviour.
func layoutRTL() bool {
	return lang.IsRTLDirection()
}

// MirrorChildren reflects every visible child horizontally within totalWidth so
// that a left-to-right arrangement becomes its right-to-left mirror. A child
// occupying [x, x+width) on the left is moved to the equivalent span measured
// from the right edge; vertical positions and all sizes are preserved.
// Full-width children (such as VBox rows or Border top/bottom bars) map onto
// themselves, so the reflection is a no-op for them.
//
// It is applied as a post-pass after a layout has positioned its children,
// keeping the original (left-to-right) positioning code untouched. The standard
// layouts call it directly; custom-renderer widgets reach it via the widget
// package's mirrorIfRTL helper.
func MirrorChildren(objects []core.CanvasObject, totalWidth float32) {
	for _, child := range objects {
		if !child.Visible() {
			continue
		}
		p := child.Position()
		child.Move(geo.NewPos(totalWidth-p.X-child.Size().Width, p.Y))
	}
}
