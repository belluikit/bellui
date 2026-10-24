package driver

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/uidir"
	"github.com/belluikit/bellui/lang"
)

// DirectionOf reports the layout direction that applies to obj where it sits:
// the innermost direction scope enclosing it on its canvas, else the canvas'
// own direction (Canvas.SetDirection). ok is false when neither applies, or obj
// is on no canvas, and the caller falls back to the process-global direction.
// A scope's own direction applies to its content, not to itself, so for a
// scope this is the direction it sits in.
//
// A layout or paint walk pushes the scopes it crosses and a direction-set
// canvas (see uidir), so inside one of those the ambient direction is returned
// without a lookup. Elsewhere — an event handler, a Refresh from a timer or a
// data binding, or a walk of an automatic canvas outside any scope — it finds
// obj's canvas and, once a scope has been rendered (uidir.ScopesInUse), walks
// that canvas' tree to obj collecting the scopes on the way: a cost
// proportional to the tree, so call it once per event or Refresh, never per
// frame (#696).
func DirectionOf(obj core.CanvasObject) (bellui.Direction, bool) {
	if d, ok := uidir.Current(); ok {
		return d, true
	}
	if obj == nil {
		return bellui.DirectionAuto, false
	}
	if c := cache.GetCanvasForObject(obj); c != nil {
		// The layout walk registered obj on c: it is on that canvas, though
		// perhaps outside its content and overlays (a window's menu bar).
		if uidir.ScopesInUse() {
			if d, found := scopeIn(c, obj); found && d != bellui.DirectionAuto {
				return d, true
			}
		}
		return canvasDirection(c)
	}

	// Not registered by a driver's layout walk (the test canvas registers
	// nothing, and an object not yet laid out is on no canvas): look for obj
	// in each window, since the driver's CanvasForObject may only guess. Skip
	// the search when every answer would be the global direction anyway.
	a := bellui.CurrentApp()
	if a == nil || a.Driver() == nil {
		return bellui.DirectionAuto, false
	}
	var canvases []core.Canvas
	directed := false
	for _, w := range a.Driver().AllWindows() {
		if c := w.Canvas(); c != nil && c.Content() != nil {
			canvases = append(canvases, c)
			_, ok := canvasDirection(c)
			directed = directed || ok
		}
	}
	if !directed && !uidir.ScopesInUse() {
		return bellui.DirectionAuto, false
	}
	for _, c := range canvases {
		if d, found := scopeIn(c, obj); found {
			if d != bellui.DirectionAuto {
				return d, true
			}
			return canvasDirection(c)
		}
	}
	return bellui.DirectionAuto, false // on no window: the global direction
}

// ResolvedDirectionOf is [DirectionOf] resolved to a concrete direction,
// falling back to the process-global one: always DirectionLTR or DirectionRTL.
func ResolvedDirectionOf(obj core.CanvasObject) bellui.Direction {
	if d, ok := DirectionOf(obj); ok {
		return d
	}
	return GlobalDirection()
}

// GlobalDirection is the direction lang.IsRTLDirection reports, as a concrete
// direction: outside a walk, the process-global one.
func GlobalDirection() bellui.Direction {
	if lang.IsRTLDirection() {
		return bellui.DirectionRTL
	}
	return bellui.DirectionLTR
}

// canvasDirection is c's own direction (Canvas.SetDirection); ok is false
// when c has none.
func canvasDirection(c core.Canvas) (bellui.Direction, bool) {
	if dc, ok := c.(interface{ Direction() bellui.Direction }); ok {
		if d := dc.Direction(); d != bellui.DirectionAuto {
			return d, true
		}
	}
	return bellui.DirectionAuto, false
}

// scopeIn looks obj up in c's overlays, then its content, and returns the
// innermost concrete direction scope enclosing it (DirectionAuto when none
// does); found is false when obj is in neither. The overlays go first: they
// are small, and an object in them (a popup) would otherwise wait on a walk of
// the whole content.
func scopeIn(c core.Canvas, obj core.CanvasObject) (bellui.Direction, bool) {
	var roots []core.CanvasObject
	if o := c.Overlays(); o != nil {
		roots = append(roots, o.List()...)
	}
	roots = append(roots, c.Content())
	for _, root := range roots {
		if root == nil {
			continue
		}
		if d, found := scopeOnPath(root, obj); found {
			return d, true
		}
	}
	return bellui.DirectionAuto, false
}

// scopeOnPath walks root's tree to obj and returns the innermost concrete
// direction scope enclosing obj (DirectionAuto when none does); found is false
// when obj is not in the tree.
func scopeOnPath(root, obj core.CanvasObject) (dir bellui.Direction, found bool) {
	var stack []bellui.Direction
	before := func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
		if o == obj {
			found = true
			if len(stack) > 0 {
				dir = stack[len(stack)-1]
			}
			return true
		}
		if ds, ok := o.(directionScoped); ok && ds.ScopedDirection() != bellui.DirectionAuto {
			stack = append(stack, ds.ScopedDirection())
		}
		return false
	}
	after := func(o core.CanvasObject, _ geo.Position, _ core.CanvasObject) {
		if ds, ok := o.(directionScoped); ok && ds.ScopedDirection() != bellui.DirectionAuto && len(stack) > 0 {
			stack = stack[:len(stack)-1]
		}
	}
	WalkCompleteObjectTree(root, before, after)
	return dir, found
}

// IsRTLAt reports whether obj sits in a right-to-left layout: [DirectionOf],
// else the process-global direction (lang.IsRTLDirection). It is the reading
// for code outside a layout or paint walk — an event handler, a Refresh that
// re-lays-out — where lang.IsRTLDirection alone sees only the global direction.
func IsRTLAt(obj core.CanvasObject) bool {
	return ResolvedDirectionOf(obj) == bellui.DirectionRTL
}
