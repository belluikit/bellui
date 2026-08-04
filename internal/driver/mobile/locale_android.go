//go:build android

package mobile

import "github.com/belluikit/bellui/lang"

// resolveSystemLocale picks up a system locale change on foregrounding. A
// locale switched while the app was backgrounded recreates the activity but
// not the process (the manifest does not list locale in configChanges), so
// lang's init never re-runs; re-resolving on the focus crossing applies the
// new locale and fires the locale-change relayout when it differs (#282).
// The override guard inside lang keeps an explicit SetLocale untouched.
func resolveSystemLocale() {
	lang.ResolveSystemLocale()
}
