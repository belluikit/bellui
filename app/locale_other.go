//go:build !android

package app

// resolveStartupLocale is a no-op away from Android: every other platform's
// system locale lookup already works during package init, so the locale is
// correct before app creation (see the android counterpart and #282).
func resolveStartupLocale() {}
