package cache

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/async"
)

var canvases async.Map[core.CanvasObject, *canvasInfo]

// GetCanvasForObject returns the canvas for the specified object.
func GetCanvasForObject(obj core.CanvasObject) core.Canvas {
	cinfo, ok := canvases.Load(obj)
	if cinfo == nil || !ok {
		return nil
	}
	cinfo.setAlive()
	return cinfo.canvas
}

// SetCanvasForObject sets the canvas for the specified object.
// The passed function will be called if the item was not previously attached to this canvas.
// An object registered to another canvas is moved to this one: it is shown here
// now, so refreshes and focus must reach this canvas (#448).
func SetCanvasForObject(obj core.CanvasObject, c core.Canvas, setup func()) {
	setCanvasForObject(obj, c, setup)
}

// CanvasRegistration is the entry the canvas map holds for an object, as
// RegisterCanvasForObject returns it. A walk that keeps it can skip the map
// lookup while Current reports true (#487).
type CanvasRegistration struct{ info *canvasInfo }

// Current reports whether the entry still maps its object to c and has not
// expired. Such an entry cannot have been deleted: Clean deletes only entries
// that expired before the clock sample it cuts off at, which is never later
// than the one read here, and CleanCanvas only the entries of a canvas being
// closed, whose walks end with it. Nor replaced: moving an object to another
// canvas expires the entry it replaces.
func (r CanvasRegistration) Current(c core.Canvas) bool {
	return r.info != nil && r.info.canvas == c && r.info.expires.Load() >= cachedNow.Load()
}

// RegisterCanvasForObject is SetCanvasForObject, and returns the entry the map
// holds for obj afterwards.
func RegisterCanvasForObject(obj core.CanvasObject, c core.Canvas, setup func()) CanvasRegistration {
	return CanvasRegistration{setCanvasForObject(obj, c, setup)}
}

func setCanvasForObject(obj core.CanvasObject, c core.Canvas, setup func()) *canvasInfo {
	// Every min-size walk calls this for every object it visits, nearly always
	// already registered to c: answer that from a Load, which does not
	// allocate, instead of building an entry LoadOrStore then discards (#435).
	old, found := canvases.Load(obj)
	if found && old.canvas == c {
		return old
	}

	cinfo := &canvasInfo{canvas: c}
	cinfo.setAlive()
	if !found {
		if old, found = canvases.LoadOrStore(obj, cinfo); found && old.canvas == c {
			return old // registered here meanwhile
		}
	}
	if found {
		// The object moved to c while the map still names the canvas it left,
		// which is open, so neither CleanCanvas nor expiry has dropped it.
		// Left there, its refreshes went to that canvas and every walk here
		// set it up again (#448). Expiring the old entry keeps a walk holding
		// it from taking it as current (see CanvasRegistration.Current).
		canvases.Store(obj, cinfo)
		old.expires.Store(0)
	}
	if setup != nil {
		setup()
	}
	return cinfo
}

type canvasInfo struct {
	expiringCache
	canvas core.Canvas
}
