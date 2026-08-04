package driver

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	idriver "github.com/belluikit/bellui/internal/driver"
)

// GeometryForObject answers, in one walk of the visible object tree, the three
// questions a caller that positions something over obj has to ask together: is
// obj in the visible tree at all, where is it, and what clips it.
//
// ok is false when obj is not found — it is absent from the canvas, hidden, or
// has a hidden ancestor — and the other results are then meaningless. pos is
// obj's absolute position in canvas-tree coordinates. clip is the bounds of
// obj's nearest clipping ancestor, in the same coordinates, with clipped true
// only when there is one; an unclipped obj is drawn against the whole canvas.
//
// It is the combined form of [AncestorsOfObject] (used for its ok alone),
// [ClipBoundsForObject] and an absolute-position lookup, which each re-derive
// the same root→obj path. Prefer it on a per-frame path — the visible-tree
// walk already carries position and clip, so answering all three at once costs
// one traversal and no path bookkeeping. Those helpers remain the right choice
// when only one answer is wanted.
//
// # Coordinate space
//
// pos is the raw walked position, matching [ClipBoundsForObject]. It is
// deliberately *not* [bellui.Driver.AbsolutePositionForObject], which
// additionally subtracts the canvas [core.Canvas.InteractiveArea] inset and so
// reports a position relative to the interactive area rather than the canvas
// tree — the space [ObjectAtPosition] also lives in (#235). The two differ
// only where that inset is non-zero (mobile, and the test canvas). Reporting
// position and clip in one space is the point: a caller
// that intersects them — as [widget.NativeViewHost] does to place an OS view —
// gets a meaningless rectangle if they come from different origins.
//
// Like [AncestorsOfObject], the walk covers the canvas content and its
// overlays; objects inside a window's menu bar are not reached.
//
// Since: 1.0
func GeometryForObject(c core.Canvas, obj core.CanvasObject) (pos geo.Position, clip geo.Rectangle, clipped, ok bool) {
	if c == nil || obj == nil {
		return geo.Position{}, geo.Rectangle{}, false, false
	}

	find := func(o core.CanvasObject, objPos, clipPos geo.Position, clipSize geo.Size) bool {
		if o != obj {
			return false
		}
		pos = objPos
		if clipSize != idriver.NoClipBounds {
			clip = geo.Rectangle{Pos: clipPos, Size: clipSize}
			clipped = true
		}
		ok = true
		return true // found — stop walking
	}

	// Walk the roots without building a combined slice: this runs every frame
	// for every hosted native view, and the slice would be the allocation the
	// whole helper exists to avoid.
	if idriver.WalkVisibleObjectTree(c.Content(), find, nil) {
		return pos, clip, clipped, ok
	}
	for _, overlay := range c.Overlays().List() {
		if idriver.WalkVisibleObjectTree(overlay, find, nil) {
			return pos, clip, clipped, ok
		}
	}
	return geo.Position{}, geo.Rectangle{}, false, false
}
