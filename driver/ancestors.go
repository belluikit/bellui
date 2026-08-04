package driver

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	idriver "github.com/belluikit/bellui/internal/driver"
)

// AncestorsOfObject returns the chain of visible objects from a canvas root down
// to obj (inclusive): the first element is the canvas root that contains obj
// (its content or an overlay), the last element is obj itself, and every element
// in between is a nesting container or widget between them. ok is false when obj
// is not found in the canvas tree (not present, or not visible).
//
// It is the top-down companion to [ObjectAtPosition] (a click → object), letting
// tooling answer "what encloses this object?" — e.g. testing whether any ancestor
// is an opt-out wrapper — reusing the same visible-tree walk the painter and
// hit-test use, so it descends both containers and widget-renderer internals.
//
// Since: 1.0
func AncestorsOfObject(c core.Canvas, obj core.CanvasObject) ([]core.CanvasObject, bool) {
	if c == nil || obj == nil {
		return nil, false
	}

	var path, stack []core.CanvasObject
	before := func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
		stack = append(stack, o)
		if o == obj {
			path = append([]core.CanvasObject(nil), stack...)
			return true // found — abort the walk
		}
		return false
	}
	after := func(core.CanvasObject, geo.Position, core.CanvasObject) {
		stack = stack[:len(stack)-1]
	}

	roots := append([]core.CanvasObject{c.Content()}, c.Overlays().List()...)
	for _, root := range roots {
		stack = stack[:0]
		if idriver.WalkVisibleObjectTree(root, before, after) {
			return path, true
		}
	}
	return nil, false
}
