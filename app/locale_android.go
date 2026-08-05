//go:build android

package app

import "github.com/belluikit/bellui/lang"

// resolveStartupLocale re-runs the system locale lookup at app creation.
// On Android the lang package's own init resolves the locale during the
// shared library's global constructor, before the JVM is attached, so it
// deterministically fails and settles on the English fallback (#282). By the
// time an app constructs its bellui.App the JNI context is available
// (ANativeActivity_onCreate stores it before calling the Go main), and app
// creation precedes any widget construction — so resolving here means UI
// built in main, including lang.L strings captured at build time, sees the
// real system locale from the start.
func resolveStartupLocale() {
	lang.ResolveSystemLocale()
}
