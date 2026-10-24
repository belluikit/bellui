package cache

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/async"
)

var renderers async.Map[core.Widget, *rendererInfo]

type isBaseWidget interface {
	ExtendBaseWidget(core.Widget)
	super() core.Widget
}

// Renderer looks up the render implementation for a widget.
// If one does not exist, it creates and caches a renderer for the widget.
// If two goroutines race to create the renderer, the loser's renderer
// is destroyed to prevent leaks.
func Renderer(wid core.Widget) core.WidgetRenderer {
	renderer, ok := CachedRenderer(wid)
	if !ok && wid != nil {
		renderer = wid.CreateRenderer()
		rinfo := &rendererInfo{renderer: renderer}
		rinfo.setAlive()
		if existing, loaded := renderers.LoadOrStore(wid, rinfo); loaded {
			// Another goroutine created a renderer first — destroy ours.
			renderer.Destroy()
			rinfo = existing
			renderer = rinfo.renderer
		}
	}

	return renderer
}

// CachedRenderer looks up the cached render implementation for a widget
// If a renderer does not exist in the cache, it returns nil, false.
func CachedRenderer(wid core.Widget) (core.WidgetRenderer, bool) {
	if wid == nil {
		return nil, false
	}

	if wd, ok := wid.(isBaseWidget); ok {
		if wd.super() != nil {
			wid = wd.super()
		}
	}

	rinfo, ok := renderers.Load(wid)
	if !ok {
		return nil, false
	}

	rinfo.setAlive()
	return rinfo.renderer, true
}

// DestroyRenderer frees a render implementation for a widget.
// This is typically for internal use only.
func DestroyRenderer(wid core.Widget) {
	rinfo, ok := renderers.LoadAndDelete(wid)
	if !ok {
		return
	}
	if rinfo != nil {
		rinfo.renderer.Destroy()
	}
	DeleteOverride(wid)
}

// IsRendered returns true of the widget currently has a renderer.
// One will be created the first time a widget is shown but may be removed after it is hidden.
func IsRendered(wid core.Widget) bool {
	_, found := renderers.Load(wid)
	return found
}

type rendererInfo struct {
	expiringCache
	renderer core.WidgetRenderer
}

// RenderedWidgets returns the widgets that currently have a cached renderer.
// Tests and benchmarks use it to release, at the end, the renderers they
// created: a test canvas never registers its objects, so closing a test window
// leaves them cached for ValidDuration, and a benchmark that builds a large
// widget per run piled them up by the gigabyte (#508).
func RenderedWidgets() map[core.Widget]struct{} {
	ws := make(map[core.Widget]struct{})
	renderers.Range(func(w core.Widget, _ *rendererInfo) bool {
		ws[w] = struct{}{}
		return true
	})
	return ws
}
