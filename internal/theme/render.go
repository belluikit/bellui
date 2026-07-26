package theme

import (
	"sync"
	"sync/atomic"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/thread"
)

// goroutineStacks holds per-goroutine theme stacks.
// Each goroutine only accesses its own key, so there is no contention
// on individual entries. sync.Map handles concurrent access to different keys.
var goroutineStacks sync.Map // map[uint64][]bellui.Theme

// activeOverrides counts the theme overrides pushed across all goroutines. It
// lets CurrentlyRenderingWithFallback skip the cost of identifying the current
// goroutine (thread.GoroutineID calls the expensive runtime.Stack) when no
// ThemeOverride is in effect at all — the overwhelmingly common case. This read
// runs once per theme lookup, i.e. once per text measurement, so it dominates
// large layouts: SetText of a 10k-line Entry made ~10k runtime.Stack calls (#138).
var activeOverrides atomic.Int64

// CurrentlyRenderingWithFallback returns the theme that is currently being used during rendering or layout
// calculations. If there is no override in effect then the fallback is returned.
func CurrentlyRenderingWithFallback(f bellui.Theme) bellui.Theme {
	if activeOverrides.Load() == 0 {
		return f // no ThemeOverride anywhere — skip the GoroutineID/runtime.Stack cost
	}
	id := thread.GoroutineID()
	if val, ok := goroutineStacks.Load(id); ok {
		stack := val.([]bellui.Theme)
		if len(stack) > 0 {
			return stack[len(stack)-1]
		}
	}

	return f
}

// PushRenderingTheme is used by the ThemeOverride container to stack the current theme during rendering
// and calculations.
func PushRenderingTheme(th bellui.Theme) {
	id := thread.GoroutineID()
	var stack []bellui.Theme
	if val, ok := goroutineStacks.Load(id); ok {
		stack = val.([]bellui.Theme)
	}
	goroutineStacks.Store(id, append(stack, th))
	activeOverrides.Add(1)
}

// PopRenderingTheme is used by the ThemeOverride container to remove an overridden theme during rendering
// and calculations.
func PopRenderingTheme() {
	id := thread.GoroutineID()
	val, ok := goroutineStacks.Load(id)
	if !ok {
		return
	}
	activeOverrides.Add(-1)
	stack := val.([]bellui.Theme)
	if len(stack) <= 1 {
		goroutineStacks.Delete(id)
		return
	}
	stack[len(stack)-1] = nil
	goroutineStacks.Store(id, stack[:len(stack)-1])
}
