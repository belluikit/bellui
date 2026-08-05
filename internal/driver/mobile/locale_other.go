//go:build !android

package mobile

// resolveSystemLocale is a no-op away from Android. iOS terminates and
// relaunches an app when the system language changes, and the other
// platforms resolve their locale correctly at package init, so only Android
// needs the driver's re-resolve pickups — the foregrounding crossing (#282)
// and the in-place config-change event (#288).
func resolveSystemLocale() {}
