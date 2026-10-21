package driver

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	idriver "github.com/belluikit/bellui/internal/driver"
)

// ClipBoundsForObject returns the region obj's clipping ancestors leave visible
// within canvas c, in absolute canvas coordinates, with ok=true when obj is
// clipped: the bounds of the nearest clipping ancestor, intersected with those
// of the clipping ancestors above it, so a scroll nested in a scroll reports
// only what the outer one shows. The region is empty when they do not
// overlap. When obj is drawn against the full canvas (no clipping ancestor) ok
// is false.
//
// A clipping ancestor is one a scroll container (or any [event.Scrollable]), a
// [bellui.Container] with ClipToBounds, or a clipping widget introduces. It is
// useful for sizing floating content — a popover, completion list or tooltip —
// to the region that will actually be visible, instead of overflowing into an
// area an ancestor scroll then clips.
//
// Since: 1.0
func ClipBoundsForObject(c core.Canvas, obj core.CanvasObject) (geo.Rectangle, bool) {
	if c == nil || obj == nil {
		return geo.Rectangle{}, false
	}
	roots := append([]core.CanvasObject{c.Content()}, c.Overlays().List()...)
	return idriver.ClipBoundsForObject(obj, roots)
}
