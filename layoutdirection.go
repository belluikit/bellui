package bellui

import "sync/atomic"

// layoutDirection holds the process-wide layout direction setting. The zero
// value is DirectionAuto, so by default the direction follows the locale.
var layoutDirection atomic.Int32

// layoutDirectionRefresh is set by the app package (via RegisterLayoutDirectionRefresh)
// so that SetLayoutDirection can re-lay-out and repaint running windows when the
// direction changes at runtime. It is nil when no app is running, in which case
// a change simply takes effect for UI built or refreshed afterwards.
var layoutDirectionRefresh atomic.Pointer[func()]

// SetLayoutDirection overrides the process-wide user-interface direction. It
// governs both layout mirroring (horizontal containers and leading/trailing
// alignment) and the default base writing direction of text — a [TextStyle]
// left at [DirectionAuto] follows this setting. Passing [DirectionAuto] (the
// default) restores locale-based detection.
//
// When the direction actually changes and an app is running, every running
// window is re-laid-out and repainted, the way a theme change is applied; this
// follows the same threading rules as [Settings.SetTheme], so call it on the
// main goroutine. If no app is running the new direction simply takes effect
// for UI built or refreshed afterwards.
//
// Since: 2.8
func SetLayoutDirection(d Direction) {
	if Direction(layoutDirection.Swap(int32(d))) == d {
		return // unchanged: nothing to refresh
	}

	if refresh := layoutDirectionRefresh.Load(); refresh != nil && *refresh != nil {
		(*refresh)()
	}
}

// CurrentLayoutDirection returns the process-wide user-interface direction
// setting as configured by [SetLayoutDirection]. The default is
// [DirectionAuto], which defers to the active locale; use lang.IsRTLDirection
// for the resolved right-to-left boolean.
//
// Since: 2.8
func CurrentLayoutDirection() Direction {
	return Direction(layoutDirection.Load())
}

// RegisterLayoutDirectionRefresh registers the callback that [SetLayoutDirection]
// invokes when the active direction changes, so the app can re-lay-out and
// repaint its running windows. It is called automatically by the app package;
// applications do not need to call it.
func RegisterLayoutDirectionRefresh(fn func()) {
	layoutDirectionRefresh.Store(&fn)
}
