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
// The passed function will be called if the item was not previously attached to this canvas
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
// closed, whose walks end with it.
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
	// already registered: answer that from a Load, which does not allocate,
	// instead of building an entry LoadOrStore then discards (#435). Same
	// outcome as the LoadOrStore below finding it.
	if old, found := canvases.Load(obj); found {
		if old.canvas != c && setup != nil {
			setup()
		}
		return old
	}

	cinfo := &canvasInfo{canvas: c}
	cinfo.setAlive()

	old, found := canvases.LoadOrStore(obj, cinfo)
	if (!found || old.canvas != c) && setup != nil {
		setup()
	}
	if found {
		return old
	}
	return cinfo
}

type canvasInfo struct {
	expiringCache
	canvas core.Canvas
}
