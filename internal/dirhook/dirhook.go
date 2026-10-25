// Package dirhook lets code that lays objects out outside a layout walk — a
// Refresh from a timer, an animation or a binding, a Container's Add, a
// widget resized by hand — run that layout in the direction the object sits
// in (#749). The lookup lives in internal/driver, which the root package
// cannot import, so the root package and the widget bases reach it through
// the hook installed here. It imports only core, so every package can use it.
//
// While no direction other than the process-wide one can apply anywhere (no
// container.DirectionScope has been rendered, no canvas has its own
// direction), [InUse] is false and callers run their layout directly: an
// atomic load is the whole cost for an app without scopes.
package dirhook

import (
	"sync/atomic"

	"github.com/belluikit/bellui/core"
)

var (
	inUse  atomic.Bool
	epoch  atomic.Uint64
	around atomic.Pointer[func(core.CanvasObject, func())]
	moved  atomic.Pointer[func(parent, obj core.CanvasObject, added bool)]
)

// Note records that a direction other than the process-wide one may now
// apply somewhere: a direction scope was rendered, a canvas was given its own
// direction, or an overlay inherited its opener's. It stays set.
func Note() { inUse.Store(true) }

// InUse reports whether [Note] has been called. Until it has, every object is
// in the process-wide direction and [Run] has nothing to push.
func InUse() bool { return inUse.Load() }

// Changed records a change that can move an object into or out of a
// direction scope (an object removed from a container, a canvas' content or
// overlays changed, a scope's direction changed), and a frame boundary. The
// driver's direction cache is rebuilt after it.
func Changed() { epoch.Add(1) }

// Added records that obj was added to parent's children (a Container's Add),
// so it may now be inside a direction scope. The driver's direction cache
// takes obj's subtree in where it stands rather than being rebuilt, so a loop
// of Adds costs a walk of what it adds, not of every scope each time.
func Added(parent, obj core.CanvasObject) { move(parent, obj, true) }

// Removed records that obj left parent's children (a Container's Remove or
// RemoveAll), so it may have left a direction scope; see [Added].
func Removed(parent, obj core.CanvasObject) { move(parent, obj, false) }

func move(parent, obj core.CanvasObject, added bool) {
	if !InUse() {
		return // nothing is in a scope, and the cache is unused
	}
	if h := moved.Load(); h != nil {
		(*h)(parent, obj, added)
		return
	}
	Changed()
}

// SetMoved installs f as the function [Added] and [Removed] report to.
// internal/driver installs it at init.
func SetMoved(f func(parent, obj core.CanvasObject, added bool)) { moved.Store(&f) }

// Epoch returns the count [Changed] raises.
func Epoch() uint64 { return epoch.Load() }

// SetAround installs f as the function [Run] delegates to. internal/driver
// installs it at init.
func SetAround(f func(obj core.CanvasObject, run func())) { around.Store(&f) }

// Run runs f with obj's layout direction as the ambient one, when [InUse] and
// a hook is installed; otherwise it just runs f. Callers on hot paths test
// [InUse] first, so the closure they pass is only built when it is needed.
func Run(obj core.CanvasObject, f func()) {
	if h := around.Load(); h != nil && InUse() {
		(*h)(obj, f)
		return
	}
	f()
}
