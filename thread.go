package bellui

import "github.com/belluikit/bellui/internal/thread"

// IsMainGoroutine reports whether the caller is running on the main (GUI) goroutine.
//
// The desktop and mobile drivers register the main goroutine during app
// initialization, so this is meaningful in the real run path. When no main
// goroutine has been registered (e.g. in some unit-test setups), it reports true.
//
// Use this to branch between running work inline and marshalling it onto the main
// goroutine with [Do] or [DoAndWait]. [RunOnMain] composes the two for the common
// "run on main, inline if already there" case.
//
// Since: 1.0
func IsMainGoroutine() bool {
	return thread.IsMainGoroutine()
}

// RunOnMain runs fn on the main (GUI) goroutine and waits for it to complete.
// If the caller is already on the main goroutine — or no running app/driver is
// available (e.g. in unit tests) — fn runs inline. Otherwise it is dispatched via
// [DoAndWait].
//
// This is the safe primitive for code that may already be on the main goroutine:
// unlike calling [DoAndWait] directly, it never logs a "called from main goroutine"
// diagnostic or re-runs fn on a fresh goroutine. Prefer it when a single entry point
// is reached both from UI event handlers (already on main) and from background
// goroutines.
//
// Since: 1.0
func RunOnMain(fn func()) {
	if thread.IsMainGoroutine() {
		fn()
		return
	}
	a := CurrentApp()
	if a == nil || a.Driver() == nil {
		fn()
		return
	}
	DoAndWait(fn)
}

// DoAndWait is used to execute a specified function in the main Bellui runtime context.
// This is required when a background process wishes to adjust graphical elements of a running app.
// Developers should use this only from within goroutines they have created.
//
// Since: 2.6
func DoAndWait(fn func()) {
	CurrentApp().Driver().DoFromGoroutine(fn, true)
}

// Do is used to execute a specified function in the main Bellui runtime context without waiting.
// This is required when a background process wishes to adjust graphical elements of a running app.
// Developers should use this only from within goroutines they have created and when the result does not have to
// be waited for.
//
// Since: 2.6
func Do(fn func()) {
	CurrentApp().Driver().DoFromGoroutine(fn, false)
}
