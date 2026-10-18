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
	// Every paint walk calls this for every object, nearly always already
	// registered: answer that from a Load, which does not allocate, instead of
	// building an entry LoadOrStore then discards (#435). Same outcome as the
	// LoadOrStore below finding it.
	if old, found := canvases.Load(obj); found {
		if old.canvas != c && setup != nil {
			setup()
		}
		return
	}

	cinfo := &canvasInfo{canvas: c}
	cinfo.setAlive()

	old, found := canvases.LoadOrStore(obj, cinfo)
	if (!found || old.canvas != c) && setup != nil {
		setup()
	}
}

type canvasInfo struct {
	expiringCache
	canvas core.Canvas
}
