package cache

import (
	"fmt"
	"log"
	"sync"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/async"
)

var renderers async.Map[core.Widget, *rendererInfo]

// extendedBy are the base widgets' reports of the widget a base widget was
// extended by: widget.BaseWidget's and internal/widget.Base's, registered by
// their packages, as package cache cannot name their unexported fields.
var extendedBy []func(core.Widget) core.Widget

// RegisterExtendedBy adds f, which returns the widget wid's base widget was
// extended by, or nil when wid has no base widget or it was never extended.
func RegisterExtendedBy(f func(wid core.Widget) core.Widget) {
	extendedBy = append(extendedBy, f)
}

var warnedWrappers sync.Map

// warnIfWrapper logs, once per pair of types, a widget whose base widget was
// extended by another object (#1146). Such a widget is a wrapper standing in
// the tree for the widget it embeds: the walk looks its renderer up under the
// wrapper, and the widget's own methods under the widget, so each gets a
// renderer. The one the walk does not visit expires, and is released as
// replaced, while the widget is still shown.
func warnIfWrapper(wid core.Widget) {
	for _, f := range extendedBy {
		impl := f(wid)
		if impl == nil {
			continue
		}
		if impl == wid {
			return
		}
		key := fmt.Sprintf("%T in place of %T", wid, impl)
		if _, warned := warnedWrappers.LoadOrStore(key, true); !warned {
			log.Printf("bellui: a %T is in the tree in place of the %T it embeds, whose ExtendBaseWidget was "+
				"called with the %[2]T. The two get separate renderers, and the one the tree does not hold "+
				"is destroyed while it is still shown. Put the %[2]T itself in the tree, or build it without "+
				"its constructor and call ExtendBaseWidget with the %[1]T: ExtendBaseWidget keeps the first "+
				"widget it is given.", wid, impl)
		}
		return
	}
}

// ResetWrapperWarningsForTests clears the once-per-type set behind the
// wrapper warning, and clears it again when the test ends.
func ResetWrapperWarningsForTests(t interface{ Cleanup(func()) }) {
	warnedWrappers.Clear()
	t.Cleanup(func() { warnedWrappers.Clear() })
}

// Renderer looks up the render implementation for a widget.
// If one does not exist, it creates and caches a renderer for the widget.
// If two goroutines race to create the renderer, the loser's renderer
// is destroyed to prevent leaks.
func Renderer(wid core.Widget) core.WidgetRenderer {
	renderer, ok := CachedRenderer(wid)
	if !ok && wid != nil {
		warnIfWrapper(wid)
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

	rinfo, ok := renderers.Load(wid)
	if !ok {
		return nil, false
	}

	rinfo.setAlive()
	return rinfo.renderer, true
}

// PeekRenderer is [CachedRenderer] without marking the renderer as in use: a
// walk that only reads the tree (the direction cache's, #749) must not keep
// the renderers of a subtree that left its window from expiring.
func PeekRenderer(wid core.Widget) (core.WidgetRenderer, bool) {
	if wid == nil {
		return nil, false
	}

	rinfo, ok := renderers.Load(wid)
	if !ok {
		return nil, false
	}
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
