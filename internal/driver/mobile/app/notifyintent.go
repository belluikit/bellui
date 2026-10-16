package app

import "C"

import "sync/atomic"

// The Java activity (GoNativeActivity) reports a rich-notification activation —
// the body tap or an action button — through the notificationActivated native
// method, which lands here (pre-rebrand tracker #723). The PendingIntent that Android delivers
// on tap carries the notification's token and, for an action button, the action
// index (the body tap uses -1).
//
// The handler is registered by the bellui app package's Android launch path.
// Notification callbacks are session-scoped (pre-rebrand tracker #516): they fire only while the
// sending process is still running, so a payload that arrives before the handler
// is registered (a cold launch from the notification) is simply dropped — the
// callback registry of the freshly-started process is empty anyway.

var notifyActivatedHandler atomic.Pointer[func(token int64, action int)]

// SetNotificationActivatedHandler installs the callback invoked when the user
// taps a rich notification's body (action = -1) or one of its action buttons
// (action = the 0-based index). The callback runs on the JNI up-call thread; it
// must hand work to the main goroutine itself.
func SetNotificationActivatedHandler(fn func(token int64, action int)) {
	notifyActivatedHandler.Store(&fn)
}

//export onNotificationActivated
func onNotificationActivated(token C.longlong, action C.int) {
	if fn := notifyActivatedHandler.Load(); fn != nil {
		(*fn)(int64(token), int(action))
	}
}
