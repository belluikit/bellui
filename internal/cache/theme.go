package cache

import (
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/async"
)

var (
	overrides     async.Map[core.CanvasObject, *overrideScope]
	overrideCount atomic.Uint32
)

// anchorMark marks where one anchor's own entries begin: an anchor owns the
// objects stored after it until the next anchor, so destroying its renderer
// releases exactly its part of the walk, not a nested anchor's.
type anchorMark struct {
	obj   core.CanvasObject
	start int
}

type overrideScope struct {
	th      bellui.Theme
	cacheID string

	mu      sync.Mutex
	objects []core.CanvasObject // everything the scope put in the map, in walk order
	markers []anchorMark        // one per anchor (widget or container) of this scope
}

// OverrideTheme allows an app to specify that a single object should use a different theme to the app.
// This should be used sparingly to avoid a jarring user experience.
// If the object is a container it will theme the children, if it is a canvas primitive it will do nothing.
//
// Since: 1.0
func OverrideTheme(o core.CanvasObject, th bellui.Theme) {
	id := overrideCount.Add(1)
	s := &overrideScope{th: th, cacheID: strconv.Itoa(int(id))}
	overrideTheme(o, s)
}

func OverrideThemeMatchingScope(o, parent core.CanvasObject) bool {
	scope, ok := overrides.Load(parent)
	if !ok { // not overridden in parent
		return false
	}

	overrideTheme(o, scope)
	return true
}

func WidgetScopeID(o core.CanvasObject) string {
	scope, ok := overrides.Load(o)
	if !ok {
		return ""
	}

	return scope.cacheID
}

func WidgetTheme(o core.CanvasObject) bellui.Theme {
	scope, ok := overrides.Load(o)
	if !ok {
		return nil
	}

	return scope.th
}

// OverriddenObjects returns the objects that currently hold a theme override
// scope. Tests and benchmarks use it to assert that an overridden subtree that
// is built and then dropped releases its entries (#741).
func OverriddenObjects() int {
	return overrides.Len()
}

// DeleteOverride removes one entry's theme scope. When the entry is an anchor
// (a widget or container the scope was applied to), the part of the walk it
// owns goes with it: the primitives its renderer reached would otherwise stay
// in the map for the life of the process once the subtree is dropped. The
// renderer cleanup paths go through here.
func DeleteOverride(o core.CanvasObject) {
	s, ok := overrides.Load(o)
	if !ok || s == nil {
		return
	}

	s.mu.Lock()
	marked := -1
	for i, m := range s.markers {
		if m.obj == o {
			marked = i
			break
		}
	}
	if marked < 0 { // a plain object: its anchor releases it, so this is only an early drop (CleanCanvas)
		s.mu.Unlock()
		deleteIfScope(o, s)
		return
	}
	end := len(s.objects)
	if marked+1 < len(s.markers) {
		end = s.markers[marked+1].start
	}
	release := append([]core.CanvasObject(nil), s.objects[s.markers[marked].start:end]...)
	s.objects = append(s.objects[:s.markers[marked].start], s.objects[end:]...)
	shift := end - s.markers[marked].start
	for j := marked + 1; j < len(s.markers); j++ {
		s.markers[j].start -= shift
	}
	s.markers = append(s.markers[:marked], s.markers[marked+1:]...)
	var rest []core.CanvasObject
	if len(s.markers) == 0 {
		rest = s.objects
		s.objects = nil
	}
	s.mu.Unlock()

	for _, obj := range release {
		deleteIfScope(obj, s)
	}
	for _, obj := range rest { // nothing anchors the scope any more: clear what is left
		deleteIfScope(obj, s)
	}
}

// deleteIfScope removes o's entry only when the map still holds it for s, so a
// stale object re-overridden under a newer scope is not dropped with this one.
func deleteIfScope(o core.CanvasObject, s *overrideScope) {
	if cur, ok := overrides.Load(o); ok && cur == s {
		overrides.Delete(o)
	}
}

func (s *overrideScope) store(o core.CanvasObject, anchor bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.objects {
		if existing == o {
			return
		}
	}
	overrides.Store(o, s)
	s.objects = append(s.objects, o)
	if anchor {
		s.markers = append(s.markers, anchorMark{obj: o, start: len(s.objects) - 1})
	}
}

func overrideContainer(c *bellui.Container, s *overrideScope) {
	// A container has no renderer of its own, so it anchors its scope here and
	// CleanCanvas releases its part when the window it belongs to closes.
	s.store(c, true)
	for _, o := range c.Objects {
		overrideTheme(o, s)
	}
}

func overrideTheme(o core.CanvasObject, s *overrideScope) {
	if _, ok := o.(interface{ SetDeviceIsMobile(bool) }); ok { // ThemeOverride without the import loop
		return // do not apply this theme over a new scope
	}

	switch c := o.(type) {
	case core.Widget:
		overrideWidget(c, s)
	case *bellui.Container:
		overrideContainer(c, s)
	default:
		s.store(o, false)
	}
}

func overrideWidget(w core.Widget, s *overrideScope) {
	ResetThemeCaches()
	s.store(w, true)

	r := Renderer(w)
	if r == nil {
		return
	}

	for _, o := range r.Objects() {
		overrideTheme(o, s)
	}
}
