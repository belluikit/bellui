package driver

import (
	"sync"
	"sync/atomic"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/internal/dirhook"
	"github.com/belluikit/bellui/internal/thread"
	"github.com/belluikit/bellui/internal/uidir"
	"github.com/belluikit/bellui/lang"
)

func init() {
	dirhook.SetAround(InDirectionOf)
	dirhook.SetMoved(func(parent, obj core.CanvasObject, added bool) {
		index.moved(parent, obj, added)
		windows.moved(parent, obj, added)
	})
}

// DirectionOf reports the layout direction that applies to obj where it sits:
// the innermost direction scope enclosing it on its canvas (or the direction
// an overlay inherited from the widget that opened it), else the canvas' own
// direction (Canvas.SetDirection). ok is false when neither applies, or obj
// is on no canvas, and the caller falls back to the process-global direction.
// A scope's own direction applies to its content, not to itself, so for a
// scope this is the direction it sits in.
//
// A layout or paint walk pushes the scopes it crosses, and while a direction
// can differ anywhere it pushes the canvas' resolved direction at its root
// (see uidir), so inside a walk the ambient direction is returned without a
// lookup. Elsewhere — an event handler, a Refresh from a timer, an animation
// or a data binding — the answer comes from a cache of the objects inside
// scopes on every window, rebuilt at most once per frame and after a change
// that can move an object between scopes (dirhook.Changed); see dirIndex.
func DirectionOf(obj core.CanvasObject) (bellui.Direction, bool) {
	if d, ok := uidir.Current(); ok {
		return d, true
	}
	if obj == nil {
		return bellui.DirectionAuto, false
	}
	if uidir.ScopesInUse() {
		if d, found := index.lookup(obj); found {
			return d, true
		}
	}
	if c := cache.GetCanvasForObject(obj); c != nil {
		// The layout walk registered obj on c: it is on that canvas, though
		// perhaps outside its content and overlays (a window's menu bar).
		return canvasDirection(c)
	}

	// Not registered by a driver's layout walk (the test canvas registers
	// nothing, and an object not yet laid out is on no canvas): look for obj
	// in each window with a direction of its own, since the driver's
	// CanvasForObject may only guess. The cache of those windows' objects
	// answers most lookups (#772); a miss walks the windows themselves.
	if d, found := windows.lookup(obj); found {
		return d, true
	}
	a := bellui.CurrentApp()
	if a == nil || a.Driver() == nil {
		return bellui.DirectionAuto, false
	}
	for _, w := range a.Driver().AllWindows() {
		c := w.Canvas()
		if c == nil || c.Content() == nil {
			continue
		}
		if d, ok := canvasDirection(c); ok && onCanvas(c, obj) {
			return d, true
		}
	}
	return bellui.DirectionAuto, false // on no window: the global direction
}

// ResolvedDirectionOf is [DirectionOf] resolved to a concrete direction,
// falling back to the process-global one: always DirectionLTR or DirectionRTL.
func ResolvedDirectionOf(obj core.CanvasObject) bellui.Direction {
	if d, ok := DirectionOf(obj); ok {
		return d
	}
	return GlobalDirection()
}

// GlobalDirection is the direction lang.IsRTLDirection reports, as a concrete
// direction: outside a walk, the process-global one.
func GlobalDirection() bellui.Direction {
	if lang.IsRTLDirection() {
		return bellui.DirectionRTL
	}
	return bellui.DirectionLTR
}

// InDirectionOf runs f with obj's direction as the ambient one, so a layout f
// performs outside a layout walk — a Refresh that re-lays a widget out after
// a tap, a timer or a binding, a Container's Add, a widget resized by hand —
// mirrors as the walk would (#696, #749). Inside a walk, or another
// InDirectionOf, a direction is already ambient and f runs as it is, unless
// obj is a scope or an overlay that inherited a direction: those push their
// own, as the walk does around their subtree. While no
// direction other than the global one can apply anywhere (dirhook.InUse) it
// pushes nothing, keeping uidir's no-scope fast path for the text measured
// inside f. Otherwise it pushes the resolved direction even when that is the
// global one, so the objects f refreshes find it ambient instead of looking
// themselves up.
//
// It is the hook the root package's Container and the widget bases reach
// through internal/dirhook.
func InDirectionOf(obj core.CanvasObject, f func()) {
	if !dirhook.InUse() {
		f()
		return
	}
	// A scope, or an overlay that inherited its opener's direction, lays its
	// content out in its own direction, not the one it sits in, whatever
	// direction is ambient where it is refreshed.
	d, ok := ScopeDirection(obj)
	if !ok {
		if _, ok := uidir.Current(); ok {
			f()
			return
		}
		d = ResolvedDirectionOf(obj)
	}
	uidir.Push(d)
	defer uidir.Pop(d)
	f()
}

// RootDirection is the direction a walk of a canvas whose own direction is
// d pushes at its root: d itself, or, while a direction other than the global
// one can apply somewhere (dirhook.InUse), the resolved global direction, so
// that a Refresh or Resize inside the walk finds a direction ambient and
// never looks itself up (#760). DirectionAuto pushes nothing, keeping uidir's
// no-scope fast path for an app without scopes.
func RootDirection(d bellui.Direction) bellui.Direction {
	if d != bellui.DirectionAuto || !dirhook.InUse() {
		return d
	}
	return GlobalDirection()
}

// InheritDirection makes overlay, which a widget is about to show, take the
// direction opener sits in: a popup opened from inside a DirectionScope lays
// out and paints in the scope's direction although overlays are outside the
// content tree (#749). Nothing is recorded when opener is in the global
// direction. The overlay stack forgets the overlay when it is removed.
func InheritDirection[T any, P interface {
	*T
	core.CanvasObject
}](overlay P, opener core.CanvasObject,
) {
	if overlay == nil {
		return
	}
	if !dirhook.InUse() || opener == nil {
		uidir.ForgetInherited(overlay)
		return
	}
	d, ok := DirectionOf(opener)
	if !ok {
		uidir.ForgetInherited(overlay)
		return
	}
	uidir.SetInherited(overlay, d)
}

// canvasDirection is c's own direction (Canvas.SetDirection); ok is false
// when c has none.
func canvasDirection(c core.Canvas) (bellui.Direction, bool) {
	if dc, ok := c.(interface{ Direction() bellui.Direction }); ok {
		if d := dc.Direction(); d != bellui.DirectionAuto {
			return d, true
		}
	}
	return bellui.DirectionAuto, false
}

// canvasRoots are the trees of c a direction lookup searches: its overlays,
// then its content.
func canvasRoots(c core.Canvas) []core.CanvasObject {
	var roots []core.CanvasObject
	if o := c.Overlays(); o != nil {
		roots = append(roots, o.List()...)
	}
	return append(roots, c.Content())
}

// onCanvas reports whether obj is in c's overlays or content.
func onCanvas(c core.Canvas, obj core.CanvasObject) bool {
	for _, root := range canvasRoots(c) {
		if root == nil {
			continue
		}
		found := WalkCompleteObjectTree(root, func(o core.CanvasObject, _, _ geo.Position, _ geo.Size) bool {
			return o == obj
		}, nil)
		if found {
			return true
		}
	}
	return false
}

// ScopeDirection is the direction obj applies to its subtree: a direction
// scope's own, or the one an overlay inherited from its opener. ok is false
// for any other object, and for an automatic scope.
func ScopeDirection(obj core.CanvasObject) (bellui.Direction, bool) {
	if ds, ok := obj.(directionScoped); ok {
		d := ds.ScopedDirection()
		return d, d != bellui.DirectionAuto
	}
	return uidir.Inherited(obj)
}

// index is the process' direction cache; see dirIndex.
var index dirIndex

// indexBuilds counts index rebuilds and indexLookups the lookups that reached
// the index, for tests and benchmarks.
var indexBuilds, indexLookups atomic.Int64

// dirIndex caches, for every object inside a direction scope (or an overlay
// that inherited a direction), the direction of its innermost scope. A lookup
// outside a walk is then a map read instead of a walk of the tree (#749).
//
// It is rebuilt — a walk of the registered scopes' subtrees, see build — on
// the first lookup after dirhook.Epoch moved: at each frame
// (common.Canvas.EnsureMinSize, the test canvas' resize), and when a canvas'
// content or overlays change, a scope's direction or content changes or an
// overlay inherits one. An object added to or removed from a Container is
// taken in or out where it stands instead (moved), so a loop of Adds does not
// rebuild it each time. An object outside every scope is not in it, so a miss
// means the canvas' or the global direction. Between rebuilds an object a
// widget's renderer moved into a scope on its own (not through a Container)
// is missed until the next frame; its parent's Refresh and the layout walk
// push the scope's direction around it in the meantime.
type dirIndex struct {
	mu      sync.Mutex
	epoch   uint64
	valid   bool
	dirs    map[core.CanvasObject]bellui.Direction
	builder atomic.Uint64 // goroutine walking for the index, 0 when none
}

// lookup returns the direction of obj's innermost scope; found is false when
// obj is in no scope on any window.
func (x *dirIndex) lookup(obj core.CanvasObject) (bellui.Direction, bool) {
	indexLookups.Add(1)
	if x.walking() {
		// A renderer asked for a direction while the index was being walked
		// on this goroutine: the walk does not create renderers, so this is
		// rare, and the answer falls back rather than deadlock.
		return bellui.DirectionAuto, false
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if e := dirhook.Epoch(); !x.valid || x.epoch != e {
		x.epoch, x.valid = e, true
		x.builder.Store(thread.GoroutineID())
		x.build()
		x.builder.Store(0)
	}
	d, ok := x.dirs[obj]
	return d, ok
}

// walking reports whether the calling goroutine is walking for the index.
func (x *dirIndex) walking() bool {
	b := x.builder.Load()
	return b != 0 && b == thread.GoroutineID()
}

// moved takes obj's subtree into the index after it was added to parent
// (added), or out of it after it left parent: dirhook.Added and Removed. An
// index already stale is left for the next lookup to rebuild.
func (x *dirIndex) moved(parent, obj core.CanvasObject, added bool) {
	if obj == nil {
		return
	}
	if x.walking() {
		dirhook.Changed() // reached from inside our own walk: rebuild instead
		return
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if !x.valid || x.epoch != dirhook.Epoch() {
		return
	}
	x.builder.Store(thread.GoroutineID())
	defer x.builder.Store(0)
	if !added {
		x.forget(obj)
		return
	}
	d, ok := ScopeDirection(parent)
	if !ok {
		d, ok = x.dirs[parent]
	}
	if ok {
		x.walk(obj, d, true)
	}
}

// build fills x.dirs from the subtrees of the registered scopes and of the
// overlays that inherited a direction. An object outside every scope is never
// in the index, so only those subtrees are walked, whatever the size of the
// app. A scope's subtree takes its direction wherever the scope sits; a
// nested scope's subtree is walked again from its own entry, with the same
// result, and the outer walk records the nested scope itself.
func (x *dirIndex) build() {
	indexBuilds.Add(1)
	if x.dirs == nil {
		x.dirs = map[core.CanvasObject]bellui.Direction{}
	} else {
		clear(x.dirs)
	}
	for _, scope := range uidir.Scopes() {
		x.walk(scope, bellui.DirectionAuto, false)
	}
}

// walk records o, when in a scope (in) whose direction is d, and its subtree.
// It reads the tree the renderers already hold and creates none, and it does
// not mark them as in use (cache.PeekRenderer): a scope that left its window
// must not stay alive through the renderers its rebuilds touch.
func (x *dirIndex) walk(o core.CanvasObject, d bellui.Direction, in bool) {
	if in {
		x.dirs[o] = d
	}
	if sd, ok := ScopeDirection(o); ok {
		d, in = sd, true
	}
	if !in {
		return // an automatic scope outside every other: nothing to record
	}
	for _, c := range indexChildren(o) {
		if c != nil {
			x.walk(c, d, true)
		}
	}
}

// forget drops o's subtree from the index, except what a scope inside it
// encloses: that keeps its scope's direction wherever o goes.
func (x *dirIndex) forget(o core.CanvasObject) {
	delete(x.dirs, o)
	if _, ok := ScopeDirection(o); ok {
		return
	}
	for _, c := range indexChildren(o) {
		if c != nil {
			x.forget(c)
		}
	}
}

// indexChildren are o's children as the layout walk sees them, from the
// renderer already cached for a widget.
func indexChildren(o core.CanvasObject) []core.CanvasObject {
	switch co := o.(type) {
	case *bellui.Container:
		return co.Objects
	case core.Widget:
		if r, ok := cache.PeekRenderer(co); ok {
			return r.Objects()
		}
	}
	return nil
}

// windows is the process' cache of the objects on windows with a direction
// of their own; see windowIndex.
var windows windowIndex

// windowIndex caches, for every object in the content or overlays of a
// window whose canvas has its own direction, that direction. It serves
// DirectionOf for objects no driver walk registered on a canvas — every
// object on the test canvas, an object not yet laid out — which otherwise
// walk every such window per lookup: on the test canvas a Refresh of each of
// 1000 labels walked the whole window (#772).
//
// It is built on the first lookup that reaches it after dirhook.Epoch moved,
// so a driver whose walk registers its objects never builds it, and kept
// current through a Container's Add and Remove as dirIndex is. Between
// rebuilds an object a renderer put on a window on its own is missed, and the
// caller walks the windows for it; one a renderer dropped keeps its window's
// direction, which applies to nothing until it is shown again.
type windowIndex struct {
	mu    sync.Mutex
	epoch uint64
	valid bool
	dirs  map[core.CanvasObject]bellui.Direction
}

// lookup returns the direction of the window obj is on; found is false when
// obj is on no window with a direction of its own, as far as the cache knows.
func (x *windowIndex) lookup(obj core.CanvasObject) (bellui.Direction, bool) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if e := dirhook.Epoch(); !x.valid || x.epoch != e {
		x.epoch, x.valid = e, true
		x.build()
	}
	d, ok := x.dirs[obj]
	return d, ok
}

// build fills x.dirs from the overlays and content of every window whose
// canvas has a direction of its own.
func (x *windowIndex) build() {
	if x.dirs == nil {
		x.dirs = map[core.CanvasObject]bellui.Direction{}
	} else {
		clear(x.dirs)
	}
	a := bellui.CurrentApp()
	if a == nil || a.Driver() == nil {
		return
	}
	for _, w := range a.Driver().AllWindows() {
		c := w.Canvas()
		if c == nil || c.Content() == nil {
			continue
		}
		d, ok := canvasDirection(c)
		if !ok {
			continue
		}
		for _, root := range canvasRoots(c) {
			if root != nil {
				x.walk(root, d)
			}
		}
	}
}

// walk records o and its subtree as on a window whose direction is d.
func (x *windowIndex) walk(o core.CanvasObject, d bellui.Direction) {
	x.dirs[o] = d
	for _, c := range indexChildren(o) {
		if c != nil {
			x.walk(c, d)
		}
	}
}

// forget drops o's subtree from the cache.
func (x *windowIndex) forget(o core.CanvasObject) {
	delete(x.dirs, o)
	for _, c := range indexChildren(o) {
		if c != nil {
			x.forget(c)
		}
	}
}

// moved takes obj's subtree in after it was added to parent, when parent is
// on a directed window, or out after it left parent; see dirIndex.moved. A
// cache already stale is left for the next lookup to rebuild.
func (x *windowIndex) moved(parent, obj core.CanvasObject, added bool) {
	if obj == nil {
		return
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	if !x.valid || x.epoch != dirhook.Epoch() {
		return
	}
	if !added {
		x.forget(obj)
		return
	}
	if d, ok := x.dirs[parent]; ok {
		x.walk(obj, d)
	}
}

// DirectionIndexStats reports how many times the direction cache was rebuilt,
// and how many lookups reached it, in this process; benchmarks and tests read
// it.
func DirectionIndexStats() (builds, lookups int64) {
	return indexBuilds.Load(), indexLookups.Load()
}

// IsRTLAt reports whether obj sits in a right-to-left layout: [DirectionOf],
// else the process-global direction (lang.IsRTLDirection). It is the reading
// for code outside a layout or paint walk — an event handler, a Refresh that
// re-lays-out — where lang.IsRTLDirection alone sees only the global direction.
func IsRTLAt(obj core.CanvasObject) bool {
	return ResolvedDirectionOf(obj) == bellui.DirectionRTL
}
