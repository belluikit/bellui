//go:build !android

package audio

// suspendWhenIdle is false outside Android: see idleSuspend.
var suspendWhenIdle = false
