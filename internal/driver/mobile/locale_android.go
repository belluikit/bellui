//go:build android && !wasm

package mobile

import "github.com/belluikit/bellui/lang"

// resolveSystemLocale picks up a system locale change from the driver's two
// pickup points: the StageFocused foregrounding crossing (a system language
// switched while backgrounded leaves the process alive — whether the activity
// was recreated or reconfigured in place — so lang's init never re-runs; #282)
// and the in-place config-change event (an Android 13+ per-app language change
// arrives as onConfigurationChanged with no lifecycle crossing at all, since
// the manifest declares locale|layoutDirection; #287, #288). Re-resolving
// applies the new locale and fires the locale-change relayout when it differs.
// The override guard inside lang keeps an explicit SetLocale untouched.
func resolveSystemLocale() {
	lang.ResolveSystemLocale()
}
