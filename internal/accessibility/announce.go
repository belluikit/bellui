//go:build accessibility

package accessibility

import "sync"

var (
	announceMu        sync.Mutex
	announceListeners []*func(text string, assertive bool)
)

// Announce pushes a transient status message to every registered screen-reader
// backend (a "live region" announcement, pre-rebrand tracker #525). Widgets that show text the
// user did not navigate to — a Toast, a Snackbar — call it when the message
// appears, because assistive technology would otherwise only discover the
// node if the user happened to walk the tree before it auto-dismisses.
//
// assertive requests that the message interrupt what the screen reader is
// currently speaking; polite (false) queues it. Backends map this to their
// platform's priority vocabulary (AXAnnouncementPriority, UIA
// NotificationProcessing, aria-live politeness).
//
// Call it on the main goroutine: backends deliver to platform APIs that are
// main-thread-only, in the same dispatch model as the tree updates.
// Announce is a no-op when the toolkit is built without the accessibility
// tag, or while no backend listener is registered.
func Announce(text string, assertive bool) {
	if text == "" {
		return
	}
	announceMu.Lock()
	listeners := make([]*func(text string, assertive bool), len(announceListeners))
	copy(listeners, announceListeners)
	announceMu.Unlock()

	for _, l := range listeners {
		(*l)(text, assertive)
	}
}

// AddAnnouncementListener registers a backend callback for [Announce] and
// returns a function that removes it again. Platform backends register once
// at startup; the callback runs on the goroutine Announce was called from
// (the main goroutine, per the Announce contract).
func AddAnnouncementListener(fn func(text string, assertive bool)) (remove func()) {
	key := &fn
	announceMu.Lock()
	announceListeners = append(announceListeners, key)
	announceMu.Unlock()

	return func() {
		announceMu.Lock()
		defer announceMu.Unlock()
		for i, l := range announceListeners {
			if l == key {
				announceListeners = append(announceListeners[:i], announceListeners[i+1:]...)
				return
			}
		}
	}
}
