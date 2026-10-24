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

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/dirhook"
	"github.com/belluikit/bellui/internal/thread"
)

// goroutineStacks holds per-goroutine direction stacks. Each goroutine only
// accesses its own key, so there is no contention on individual entries;
// sync.Map handles concurrent access to different keys. This mirrors the theme
// stack in internal/theme/render.go.
var goroutineStacks sync.Map // map[uint64][]bellui.Direction

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

	if inheritedCount.Load() > 0 {
		inheritedMu.Lock()
		for _, e := range inherited {
			live = append(live, e.overlay)
		}
		inheritedMu.Unlock()
	}
	return live
}

// ScopesInUse reports whether any direction scope has been rendered, or an
// overlay inherited a direction, in this process; see [NoteScope].
func ScopesInUse() bool { return scopesSeen.Load() }

// inherited holds the directions overlays take from the widget that opened
// them (a Select's popup menu, a context menu): an overlay sits outside the
// content tree, so no scope encloses it (#749). Entries are set by
// [SetInherited] when the overlay is opened and dropped by [ForgetInherited]
// when it leaves its canvas' overlay stack. It is a short slice, not a map:
// few overlays are open at once, and comparing an object against the stored
// (pointer) overlays cannot panic the way hashing an uncomparable object
// value as a map key would.
var (
	inheritedMu    sync.Mutex
	inherited      []inheritedEntry
	inheritedCount atomic.Int64
)

type inheritedEntry struct {
	overlay core.CanvasObject
	dir     bellui.Direction
}

// SetInherited makes overlay lay out and paint its subtree in d, as a
// direction scope around it would. A DirectionAuto d forgets overlay.
func SetInherited(overlay core.CanvasObject, d bellui.Direction) {
	if overlay == nil {
		return
	}
	if d == bellui.DirectionAuto {
		ForgetInherited(overlay)
		return
	}
	inheritedMu.Lock()
	found := false
	for i := range inherited {
		if inherited[i].overlay == overlay {
			inherited[i].dir, found = d, true
			break
		}
	}
	if !found {
		inherited = append(inherited, inheritedEntry{overlay, d})
		inheritedCount.Add(1)
	}
	inheritedMu.Unlock()
	NoteScope()
	dirhook.Changed()
}

// ForgetInherited drops overlay's inherited direction, if it has one.
func ForgetInherited(overlay core.CanvasObject) {
	if inheritedCount.Load() == 0 || overlay == nil {
		return
	}
	inheritedMu.Lock()
	removed := false
	for i := range inherited {
		if inherited[i].overlay == overlay {
			inherited = append(inherited[:i], inherited[i+1:]...)
			inheritedCount.Add(-1)
			removed = true
			break
		}
	}
	inheritedMu.Unlock()
	if removed {
		dirhook.Changed()
	}
}

// Inherited returns the direction obj, an overlay, took from its opener; ok
// is false when it took none. It is a single atomic load while no overlay has
// one.
func Inherited(obj core.CanvasObject) (bellui.Direction, bool) {
	if inheritedCount.Load() == 0 || obj == nil {
		return bellui.DirectionAuto, false
	}
	inheritedMu.Lock()
	defer inheritedMu.Unlock()
	for _, e := range inherited {
		if e.overlay == obj {
			return e.dir, true
		}
	}
	return bellui.DirectionAuto, false
}

// Current returns the ambient direction override in effect on the calling
// goroutine, if any. The bool is false when no scope is active, in which case
// callers fall back to the process-global direction. The returned direction is
// always concrete (LTR or RTL); see the package comment.
func Current() (bellui.Direction, bool) {
	if activeScopes.Load() == 0 {
		return bellui.DirectionAuto, false // no DirectionScope anywhere — skip GoroutineID
	}
	id := thread.GoroutineID()
	if val, ok := goroutineStacks.Load(id); ok {
		stack := val.([]bellui.Direction)
		if len(stack) > 0 {
			return stack[len(stack)-1], true
		}
	}
	return bellui.DirectionAuto, false
}

// Push installs d as the ambient direction for the calling goroutine until the
// matching [Pop]. It is called by a direction scope around the layout and paint
// of its subtree. Pushing [bellui.DirectionAuto] is a no-op so that an automatic
// scope transparently inherits the enclosing direction.
func Push(d bellui.Direction) {
	if d == bellui.DirectionAuto {
		return
	}
	id := thread.GoroutineID()
	var stack []bellui.Direction
	if val, ok := goroutineStacks.Load(id); ok {
		stack = val.([]bellui.Direction)
	}
	goroutineStacks.Store(id, append(stack, d))
	activeScopes.Add(1)
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
	id := thread.GoroutineID()
	if val, ok := goroutineStacks.Load(id); ok {
		return len(val.([]bellui.Direction))
	}
	return 0
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
	val, ok := goroutineStacks.Load(id)
	if !ok {
		return
	}
	stack := val.([]bellui.Direction)
	if len(stack) <= depth {
		return
	}
	activeScopes.Add(int64(depth - len(stack))) // discard the orphaned entries
	if depth <= 0 {
		goroutineStacks.Delete(id)
		return
	}
	for i := depth; i < len(stack); i++ {
		stack[i] = bellui.DirectionAuto // release references for GC
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
	val, ok := goroutineStacks.Load(id)
	if !ok {
		return
	}
	activeScopes.Add(-1)
	stack := val.([]bellui.Direction)
	if len(stack) <= 1 {
		goroutineStacks.Delete(id)
		return
	}
	stack[len(stack)-1] = bellui.DirectionAuto
	goroutineStacks.Store(id, stack[:len(stack)-1])
}
