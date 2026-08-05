// Package config defines the event that reports an in-place configuration
// change to the mobile driver.
//
// On Android, a configuration change the manifest declares in configChanges
// (the packaging template declares locale|layoutDirection, #287) is delivered
// to the running activity as onConfigurationChanged instead of an activity
// recreation, so no lifecycle crossing occurs that the driver could hook. The
// native callback forwards this event so the driver reacts on its own
// goroutine — re-resolving the system locale for an Android 13+ per-app
// language change (#288) — never on the Java main thread. iOS has no
// equivalent in-place delivery (a language change terminates the app), so the
// event is Android-only.
package config

// Event reports that the running activity received an in-place configuration
// change. It carries no payload: the interested consumers re-read the state
// they care about (the lang package re-resolves the locale itself via JNI).
type Event struct{}
