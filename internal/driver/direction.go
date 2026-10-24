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
// is not on a canvas, and the caller falls back to the process-global
// direction. A scope's own direction applies to its content, not to itself, so
// for a scope this is the direction it sits in.
//
// Layout and paint walks push the scopes they cross (see uidir), so inside one
// the ambient direction is returned without a lookup. Outside a walk — an event
// handler, or a Refresh from a timer or a data binding — it finds obj's canvas
// and walks that canvas' tree from its root to obj, collecting the scopes on the
// way: a cost proportional to the tree, so call it once per event or Refresh,
// never per frame (#696).
func DirectionOf(obj core.CanvasObject) (bellui.Direction, bool) {
	if d, ok := uidir.Current(); ok {
		return d, true
	}
	if obj == nil {
		return bellui.DirectionAuto, false
	}
	c := canvasFor(obj)
	if c == nil {
		return bellui.DirectionAuto, false
	}
	canvasDir := bellui.DirectionAuto
	if dc, ok := c.(interface{ Direction() bellui.Direction }); ok {
		canvasDir = dc.Direction()
	}

	if uidir.ScopesInUse() {
		roots := append([]core.CanvasObject{c.Content()}, c.Overlays().List()...)
		for _, root := range roots {
			if d, found := scopeOnPath(root, obj); found {
				if d != bellui.DirectionAuto {
					return d, true
				}
				break
			}
		}
	}
	return canvasDir, canvasDir != bellui.DirectionAuto
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

// canvasFor returns the canvas obj was last laid out on (the drivers register
// it during the layout walk), else the one the app's driver reports for it.
func canvasFor(obj core.CanvasObject) core.Canvas {
	if c := cache.GetCanvasForObject(obj); c != nil {
		return c
	}
	a := bellui.CurrentApp()
	if a == nil || a.Driver() == nil {
		return nil
	}
	c := a.Driver().CanvasForObject(obj)
	if c == nil || c.Content() == nil {
		return nil
	}
	return c
}

// IsRTLAt reports whether obj sits in a right-to-left layout: [DirectionOf],
// else the process-global direction (lang.IsRTLDirection). It is the reading
// for code outside a layout or paint walk — an event handler, a Refresh that
// re-lays-out — where lang.IsRTLDirection alone sees only the global direction.
func IsRTLAt(obj core.CanvasObject) bool {
	if d, ok := DirectionOf(obj); ok {
		return d == bellui.DirectionRTL
	}
	return lang.IsRTLDirection()
}
