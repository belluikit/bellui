package cache

import (
	"strconv"
	"sync/atomic"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/async"
)

var (
	overrides     async.Map[core.CanvasObject, *overrideScope]
	overrideCount atomic.Uint32
)

type overrideScope struct {
	th      bellui.Theme
	cacheID string
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

func overrideContainer(c *bellui.Container, s *overrideScope) {
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
		overrides.Store(c, s)
	}
}

func overrideWidget(w core.Widget, s *overrideScope) {
	ResetThemeCaches()
	overrides.Store(w, s)

	r := Renderer(w)
	if r == nil {
		return
	}

	for _, o := range r.Objects() {
		overrideTheme(o, s)
	}
}
