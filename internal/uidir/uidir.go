// Package uidir holds an ambient, per-goroutine user-interface direction
// override. It is the layout-direction analogue of the per-goroutine theme
// stack in internal/theme: a [github.com/belluikit/bellui/container.DirectionScope]
// (or a direction-scoped window canvas) pushes its direction while it lays out
// and paints its subtree, so the canvas-free layout, measurement and shaping
// code that resolves direction through lang.IsRTLDirection picks up the scoped
// value instead of the process-global one.
//
// Callers push only concrete directions (LTR or RTL); a DirectionAuto scope is
// a no-op that defers to the enclosing scope or the global setting, so the
// stack never contains DirectionAuto and [Current] is a single read of the top.
package uidir

import (
	"sync"
	"sync/atomic"
	"weak"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/dirhook"
	"github.com/belluikit/bellui/internal/thread"
)

// goroutineStacks holds per-goroutine direction stacks. Each goroutine only
// accesses its own key, so there is no contention on individual entries;
// sync.Map handles concurrent access to different keys. This mirrors the theme
// stack in internal/theme/render.go.
var goroutineStacks sync.Map // map[uint64][]entry

// activeScopes counts the direction scopes pushed across all goroutines, so
// Current/Mark can skip identifying the goroutine (thread.GoroutineID calls the
// expensive runtime.Stack) when no DirectionScope is in effect — the common
// case, hit once per direction resolution i.e. per text measurement. Mirrors the
// theme stack's activeOverrides (pre-rebrand tracker #138).
var activeScopes atomic.Int64

// scopesSeen records that a direction scope has been rendered in this process.
// Until one has, no object can sit inside a scope, so a direction lookup
// (internal/driver.DirectionOf) can skip its scope cache — the common case.
var scopesSeen atomic.Bool

// NoteScope records that a direction scope exists. A scope calls it when its
// renderer is created, which precedes it having any laid-out content.
func NoteScope() {
	scopesSeen.Store(true)
	dirhook.Note()
}

// NoteDirectedCanvas records that a canvas was given a direction of its own
// (Canvas.SetDirection), so re-layout outside a walk must ask for an object's
// direction from now on (internal/dirhook). Unlike [NoteScope] it does not
// make a lookup walk for scopes.
func NoteDirectedCanvas() { dirhook.Note() }

// scopes holds every direction scope rendered in this process, weakly: each
// entry returns its scope, or nil once the scope was collected. The direction
// cache (internal/driver) walks their subtrees rather than every window, so
// its cost follows what the scopes hold, not the size of the app (#749).
var (
	scopesMu sync.Mutex
	scopes   []func() core.CanvasObject
)

// RegisterScope records a rendered direction scope; get returns it, or nil
// once it is gone (a weak reference, so the registry keeps nothing alive).
func RegisterScope(get func() core.CanvasObject) {
	scopesMu.Lock()
	scopes = append(scopes, get)
	scopesMu.Unlock()
	NoteScope()
	dirhook.Changed()
}

// Scopes returns the registered scopes still alive, and the overlays that
// inherited a direction, dropping the scopes that are gone.
func Scopes() []core.CanvasObject {
	scopesMu.Lock()
	live := make([]core.CanvasObject, 0, len(scopes))
	kept := scopes[:0]
	for _, get := range scopes {
		if o := get(); o != nil {
			live = append(live, o)
			kept = append(kept, get)
		}
	}
	for i := len(kept); i < len(scopes); i++ {
		scopes[i] = nil
	}
	scopes = kept
	scopesMu.Unlock()

	for _, e := range loadInherited() {
		if o := e.get(); o != nil {
			live = append(live, o)
		}
	}
	pruneInherited()
	return live
}

// ScopesInUse reports whether any direction scope has been rendered, or an
// overlay inherited a direction, in this process; see [NoteScope].
func ScopesInUse() bool { return scopesSeen.Load() }

// inherited holds the directions overlays take from the widget that opened
// them (a Select's popup menu, a context menu): an overlay sits outside the
// content tree, so no scope encloses it (#749). Entries are set by
// [SetInherited] when the overlay is opened and dropped by [ForgetInherited]
// when it leaves its canvas' overlay stack. An entry holds its overlay weakly,
// so an overlay that never leaves a stack through Remove (its window closed
// with the popup open) is not kept alive by it, and its entry is pruned once
// it is gone.
//
// It is a short slice, not a map: few overlays are open at once, and comparing
// an object against the stored (pointer) overlays cannot panic the way hashing
// an uncomparable object value as a map key would. It is copied on write and
// read without a lock, since the walks read it at every node while an
// overlay has an entry; inheritedMu orders the writers.
var (
	inheritedMu   sync.Mutex
	inheritedList atomic.Pointer[[]inheritedEntry]
)

type inheritedEntry struct {
	get func() core.CanvasObject // the overlay, or nil once it was collected
	dir bellui.Direction
}

func loadInherited() []inheritedEntry {
	if l := inheritedList.Load(); l != nil {
		return *l
	}
	return nil
}

// storeInherited publishes l; callers hold inheritedMu.
func storeInherited(l []inheritedEntry) {
	if len(l) == 0 {
		inheritedList.Store(nil)
		return
	}
	inheritedList.Store(&l)
}

// SetInherited makes overlay lay out and paint its subtree in d, as a
// direction scope around it would. A DirectionAuto d forgets overlay.
func SetInherited[T any, P interface {
	*T
	core.CanvasObject
}](overlay P, d bellui.Direction,
) {
	if overlay == nil {
		return
	}
	if d == bellui.DirectionAuto {
		ForgetInherited(overlay)
		return
	}
	inheritedMu.Lock()
	old := loadInherited()
	l := make([]inheritedEntry, 0, len(old)+1)
	found := false
	for _, e := range old {
		o := e.get()
		if o == nil {
			continue // collected
		}
		if o == core.CanvasObject(overlay) {
			e.dir, found = d, true
		}
		l = append(l, e)
	}
	if !found {
		wp := weak.Make((*T)(overlay))
		l = append(l, inheritedEntry{get: func() core.CanvasObject {
			if p := wp.Value(); p != nil {
				return P(p)
			}
			return nil
		}, dir: d})
	}
	storeInherited(l)
	inheritedMu.Unlock()
	NoteScope()
	dirhook.Changed()
}

// ForgetInherited drops overlay's inherited direction, if it has one.
func ForgetInherited(overlay core.CanvasObject) {
	if inheritedList.Load() == nil || overlay == nil {
		return
	}
	inheritedMu.Lock()
	old := loadInherited()
	l := make([]inheritedEntry, 0, len(old))
	removed := false
	for _, e := range old {
		o := e.get()
		if o == nil || o == overlay {
			removed = removed || o != nil
			continue
		}
		l = append(l, e)
	}
	storeInherited(l)
	inheritedMu.Unlock()
	if removed {
		dirhook.Changed()
	}
}

// pruneInherited drops the entries whose overlay was collected.
func pruneInherited() {
	for _, e := range loadInherited() {
		if e.get() == nil {
			inheritedMu.Lock()
			old := loadInherited()
			l := make([]inheritedEntry, 0, len(old))
			for _, e := range old {
				if e.get() != nil {
					l = append(l, e)
				}
			}
			storeInherited(l)
			inheritedMu.Unlock()
			return
		}
	}
}

// Inherited returns the direction obj, an overlay, took from its opener; ok
// is false when it took none. It is a single atomic load while no overlay has
// one, and takes no lock otherwise.
func Inherited(obj core.CanvasObject) (bellui.Direction, bool) {
	l := inheritedList.Load()
	if l == nil || obj == nil {
		return bellui.DirectionAuto, false
	}
	for _, e := range *l {
		if o := e.get(); o != nil && o == obj {
			return e.dir, true
		}
	}
	return bellui.DirectionAuto, false
}

// entry is one pushed direction. owner is the walked object that pushed it
// ([PushOwned]), nil for a plain [Push]; overlay marks an owner that pushed
// as an overlay with an inherited direction.
type entry struct {
	dir     bellui.Direction
	owner   core.CanvasObject
	overlay bool
}

// overlayPushes counts the entries pushed by overlays with an inherited
// direction across all goroutines, so [PopOwned] for an ordinary object can
// skip identifying the goroutine while none is pushed.
var overlayPushes atomic.Int64

func stackOf(id uint64) []entry {
	if val, ok := goroutineStacks.Load(id); ok {
		return val.([]entry)
	}
	return nil
}

// Current returns the ambient direction override in effect on the calling
// goroutine, if any. The bool is false when no scope is active, in which case
// callers fall back to the process-global direction. The returned direction is
// always concrete (LTR or RTL); see the package comment.
func Current() (bellui.Direction, bool) {
	if activeScopes.Load() == 0 {
		return bellui.DirectionAuto, false // no DirectionScope anywhere — skip GoroutineID
	}
	if stack := stackOf(thread.GoroutineID()); len(stack) > 0 {
		return stack[len(stack)-1].dir, true
	}
	return bellui.DirectionAuto, false
}

// Push installs d as the ambient direction for the calling goroutine until the
// matching [Pop]. It is called by a direction scope around the layout and paint
// of its subtree. Pushing [bellui.DirectionAuto] is a no-op so that an automatic
// scope transparently inherits the enclosing direction.
func Push(d bellui.Direction) { push(entry{dir: d}) }

// PushOwned pushes d for owner, an object a walk visits, until the matching
// [PopOwned] for owner. A walk pairs them in its before- and after-children
// callbacks: the pop removes what owner pushed, if anything, even when
// owner's direction changed in between (#749). overlay marks an overlay that
// pushes a direction it inherited (see [SetInherited]).
func PushOwned(d bellui.Direction, owner core.CanvasObject, overlay bool) {
	push(entry{dir: d, owner: owner, overlay: overlay})
}

func push(e entry) {
	if e.dir == bellui.DirectionAuto {
		return
	}
	id := thread.GoroutineID()
	goroutineStacks.Store(id, append(stackOf(id), e))
	activeScopes.Add(1)
	if e.overlay {
		overlayPushes.Add(1)
	}
}

// PopOwned removes the direction owner pushed with [PushOwned], if it pushed
// one: the top entry, when owner is its owner. mayBeScope is false for an
// object that is not a direction scope, which can only have pushed as an
// overlay: while no overlay has pushed, that is a single atomic load.
func PopOwned(owner core.CanvasObject, mayBeScope bool) {
	if activeScopes.Load() == 0 || (!mayBeScope && overlayPushes.Load() == 0) {
		return
	}
	id := thread.GoroutineID()
	stack := stackOf(id)
	if len(stack) == 0 || stack[len(stack)-1].owner != owner {
		return
	}
	popTop(id, stack)
}

// Mark returns the calling goroutine's current direction-stack depth. Pair it
// with [Restore] (typically via defer) around a tree walk that does per-node
// [Push]/[Pop] in separate visitor callbacks: if a draw or layout panic unwinds
// past the matching Pops, Restore discards the orphaned entries so the
// per-goroutine stack does not leak and corrupt later direction resolution.
func Mark() int {
	if activeScopes.Load() == 0 {
		return 0
	}
	return len(stackOf(thread.GoroutineID()))
}

// Restore truncates the calling goroutine's direction stack back to the depth
// returned by an earlier [Mark]. On the normal balanced-Pop path the stack is
// already at that depth, so Restore is a no-op; it only acts when a panic left
// entries pushed after the Mark.
func Restore(depth int) {
	if activeScopes.Load() == 0 {
		return
	}
	id := thread.GoroutineID()
	stack := stackOf(id)
	if len(stack) <= depth {
		return
	}
	depth = max(depth, 0)
	activeScopes.Add(int64(depth - len(stack))) // discard the orphaned entries
	for i := depth; i < len(stack); i++ {
		if stack[i].overlay {
			overlayPushes.Add(-1)
		}
		stack[i] = entry{} // release references for GC
	}
	if depth == 0 {
		goroutineStacks.Delete(id)
		return
	}
	goroutineStacks.Store(id, stack[:depth])
}

// Pop removes the direction installed by the matching [Push]. A Pop balancing a
// [bellui.DirectionAuto] Push (which pushed nothing) is a no-op.
func Pop(d bellui.Direction) {
	if d == bellui.DirectionAuto {
		return
	}
	id := thread.GoroutineID()
	stack := stackOf(id)
	if len(stack) == 0 {
		return
	}
	popTop(id, stack)
}

func popTop(id uint64, stack []entry) {
	activeScopes.Add(-1)
	if stack[len(stack)-1].overlay {
		overlayPushes.Add(-1)
	}
	if len(stack) <= 1 {
		goroutineStacks.Delete(id)
		return
	}
	stack[len(stack)-1] = entry{}
	goroutineStacks.Store(id, stack[:len(stack)-1])
}
