//go:build !ci && !mobile && !tinygo

// app_badge_ios.go supplies the iOS counterparts to app_badge_darwin.go: the
// app-icon badge via UIApplication and a no-op notification fallback (iOS apps
// are always bundled+signed, so the UserNotifications authorization path in
// app_darwin.m never falls back). See pre-rebrand tracker #739.

package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework UIKit

void setIOSBadge(long count);
*/
import "C"

// SetBadge sets the app-icon badge number, clearing it when count <= 0.
// Best-effort; part of the [bellui.BadgeApp] optional interface.
func (a *belluiApp) SetBadge(count int) {
	if count < 0 {
		count = 0
	}
	C.setIOSBadge(C.long(count))
}

// fallbackNotification is a no-op on iOS: there is no osascript, and a bundled
// iOS app obtains UserNotifications authorization through the normal path.
func fallbackNotification(title, content string) {}
