//go:build !android

package mobile

// resolveSystemLocale is a no-op away from Android. iOS terminates and
// relaunches an app when the system language changes, and the other
// platforms resolve their locale correctly at package init, so only Android
// needs the foregrounding re-resolve (#282).
func resolveSystemLocale() {}
