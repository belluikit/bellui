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
// than the one read here, CleanCanvas only the entries of a canvas being
// closed, whose walks end with it, and ReleaseObject expires the entry it
// deletes. Nor replaced: moving an object to another canvas expires the entry
// it replaces.
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

// ReleaseObject drops what the caches hold for obj on canvas c, as CleanCanvas
// does for every object of a closing canvas: its canvas-map entry and, for a
// widget, its renderer. A canvas calls it for content it replaced and no
// longer shows, which would otherwise stay until a Clean after a paint a
// minute later, and an idle app makes no such paint (#1075). An object the map
// gives to another canvas is shown there, and is left alone.
func ReleaseObject(obj core.CanvasObject, c core.Canvas) {
	if cinfo, ok := canvases.Load(obj); ok {
		if cinfo.canvas != c {
			return
		}
		canvases.Delete(obj)
		cinfo.expires.Store(0) // see CanvasRegistration.Current
	}
	if wid, ok := obj.(core.Widget); ok {
		DestroyRenderer(wid)
		return
	}
	DeleteOverride(obj)
}

// ReleaseUnkept releases, as ReleaseObject does, what the caches hold for
// every object keep does not report, except what the canvas map gives to a
// canvas other than c (#1075). c calls it when it has replaced content, with
// keep reporting what its trees still hold: the objects of the replaced
// content are then let go of at once, where a Clean waits for a paint a
// minute later. That includes what a widget holds outside its renderer's
// objects, which no walk from the replaced root reaches, such as the rows a
// list keeps for reuse or the buttons tabs measure.
//
// A renderer whose widget no canvas has registered goes too: a widget the app
// keeps off screen, such as a pop-up it shows again, then gets a new renderer
// when it is next shown, as it does today after ValidDuration unshown.
func ReleaseUnkept(c core.Canvas, keep func(core.CanvasObject) bool) {
	var gone []core.CanvasObject
	canvases.Range(func(obj core.CanvasObject, cinfo *canvasInfo) bool {
		if cinfo.canvas == c && !keep(obj) {
			gone = append(gone, obj)
		}
		return true
	})
	renderers.Range(func(wid core.Widget, _ *rendererInfo) bool {
		if _, registered := canvases.Load(wid); !registered && !keep(wid) {
			gone = append(gone, wid)
		}
		return true
	})
	// Release only after both ranges, as a renderer's Destroy may change the
	// maps.
	for _, obj := range gone {
		ReleaseObject(obj, c)
	}
}

type canvasInfo struct {
	expiringCache
	canvas core.Canvas
}
