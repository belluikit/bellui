package widget

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/lang"
	"github.com/belluikit/bellui/layout"
)

// mirrorIfRTL reflects each child horizontally within width when the layout
// direction is right-to-left. It is the custom-renderer analogue of the standard
// layouts' MirrorChildren post-pass: a widget whose renderer positions a
// horizontal sequence of children from a left origin can call this at the end of
// its Layout to obtain the mirrored arrangement, leaving the left-to-right code
// path byte-for-byte unchanged.
//
// Full-width children (backgrounds, dividers, ripple overlays) reflect onto
// themselves, so a blanket pass over all of a renderer's objects is safe.
//
// It reads the ambient direction, which is right inside a layout walk. A
// renderer that lays itself out from its own Refresh runs outside one, so it
// wraps that Layout in [inDirectionOf].
func mirrorIfRTL(objects []core.CanvasObject, width float32) {
	if lang.IsRTLDirection() {
		layout.MirrorChildren(objects, width)
	}
}

// isRTLAt reports whether obj sits in a right-to-left layout (see
// driver.IsRTLAt). Event handlers use it instead of lang.IsRTLDirection, which
// sees only the global direction outside a layout or paint walk (#696). It
// looks obj up in its canvas' tree, so it is for once-per-event use, not per
// frame.
func isRTLAt(obj core.CanvasObject) bool { return driver.IsRTLAt(obj) }

// inDirectionOf runs f with obj's direction as the ambient one, so a layout f
// performs outside a layout walk mirrors as the walk would (#696). Since #749
// BaseWidget.Refresh and Resize already run in the widget's direction, so
// this only matters for a renderer whose Refresh is reached some other way;
// see driver.InDirectionOf.
func inDirectionOf(obj core.CanvasObject, f func()) { driver.InDirectionOf(obj, f) }

// inheritDirection makes the overlay pop, about to be shown, lay out in the
// direction opener sits in: a popup opened from inside a
// container.DirectionScope takes the scope's direction although overlays sit
// outside the content tree (#749). Every widget that opens a popup of its own
// calls it before each show; the overlay stack forgets it on removal.
func inheritDirection[T any, P interface {
	*T
	core.CanvasObject
}](pop P, opener core.CanvasObject,
) {
	driver.InheritDirection(pop, opener)
}
