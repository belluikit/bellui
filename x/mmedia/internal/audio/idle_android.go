//go:build android

package audio

// suspendWhenIdle is true on Android, where an idle device's stream would be
// reopened in a phone call's audio mode (#1052).
var suspendWhenIdle = true
