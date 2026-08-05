package lang

import (
	"sync"
	"sync/atomic"
)

// localeChangeRefresh is set by the app package (via RegisterLocaleChangeRefresh)
// so that SetLocale can re-lay-out and repaint running windows when the active
// locale changes at runtime. It is nil when no app is running, in which case a
// change simply takes effect for UI built or refreshed afterwards.
var localeChangeRefresh atomic.Pointer[func()]

// localeChangeListeners holds per-subscriber callbacks registered via
// AddLocaleChangeListener. Unlike localeChangeRefresh (a single app-level
// callback that re-lays-out running windows), these let individual values —
// such as a localized data binding — re-derive their translated content when
// the active locale changes. The key is a unique id so a listener can remove
// itself without identifying the (uncomparable) func by value.
var (
	listenersMu        sync.Mutex
	localeChangeFns    = map[uint64]func(){}
	nextLocaleListener uint64
)

// RegisterLocaleChangeRefresh registers the callback that [SetLocale] — and a
// [ResolveSystemLocale] re-resolve that lands on a different locale — invokes
// when the active locale changes, so the app can re-lay-out and repaint its
// running windows (re-translating text and re-applying locale-derived formatting
// and direction). It is called automatically by the app package; applications do
// not need to call it.
//
// This mirrors [github.com/belluikit/bellui.RegisterLayoutDirectionRefresh],
// which wires the analogous refresh for an explicit layout-direction change.
func RegisterLocaleChangeRefresh(fn func()) {
	localeChangeRefresh.Store(&fn)
}

// AddLocaleChangeListener registers fn to be called whenever the active locale
// changes — through [SetLocale] or a [ResolveSystemLocale] re-resolve that
// lands on a different locale. It returns a function that removes the listener
// again; callers should invoke it when the listener is no longer needed so it
// can be garbage collected.
//
// This is the per-subscriber companion to the app-level
// [RegisterLocaleChangeRefresh]: it lets an individual value re-derive its
// locale-dependent content on a change, rather than relaying out every window.
// The canonical user is [github.com/belluikit/bellui/data/binding.NewLocalizedString].
// fn runs on whichever goroutine calls SetLocale (documented to be the main one).
//
// Since: 2.8
func AddLocaleChangeListener(fn func()) (remove func()) {
	listenersMu.Lock()
	id := nextLocaleListener
	nextLocaleListener++
	localeChangeFns[id] = fn
	listenersMu.Unlock()

	return func() {
		listenersMu.Lock()
		delete(localeChangeFns, id)
		listenersMu.Unlock()
	}
}

// refreshLocaleChange notifies per-subscriber listeners and then invokes the
// registered app-level locale-change refresh callback, if any. Listeners run
// first so that any re-derived content (e.g. translated bindings) is in place
// before the windows are re-laid-out and repainted. It is a no-op when nothing
// has subscribed and no app has registered a refresh.
func refreshLocaleChange() {
	listenersMu.Lock()
	fns := make([]func(), 0, len(localeChangeFns))
	for _, fn := range localeChangeFns {
		fns = append(fns, fn)
	}
	listenersMu.Unlock()
	for _, fn := range fns {
		fn()
	}

	if refresh := localeChangeRefresh.Load(); refresh != nil && *refresh != nil {
		(*refresh)()
	}
}
