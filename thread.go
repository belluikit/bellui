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
// On macOS 26 and later each call can wait several milliseconds for the main
// loop to wake: AppKit delivers a wake posted from another thread late, on a
// grid of about 7 ms (macOS 15 wakes in under half a millisecond). Back-to-back
// calls from one goroutine then run at about 145 a second, so batch work into
// one call rather than making many in a row. See #1033.
//
// Since: 1.0
func DoAndWait(fn func()) {
	CurrentApp().Driver().DoFromGoroutine(fn, true)
}

// Do is used to execute a specified function in the main Bellui runtime context without waiting.
// This is required when a background process wishes to adjust graphical elements of a running app.
// Developers should use this only from within goroutines they have created and when the result does not have to
// be waited for.
//
// On macOS 26 and later the function can run several milliseconds after the
// call; see [DoAndWait].
//
// Since: 1.0
func Do(fn func()) {
	CurrentApp().Driver().DoFromGoroutine(fn, false)
}
