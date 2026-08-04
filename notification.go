package bellui

// Notification represents a user notification that can be sent to the operating system.
type Notification struct {
	Title, Content string
}

// NewNotification creates a notification that can be passed to [App.SendNotification].
func NewNotification(title, content string) *Notification {
	return &Notification{Title: title, Content: content}
}

// NotificationAction is one inline action button attached to a
// [NotificationOptions]. Tapping the button in the OS notification runs
// Callback on the main goroutine.
//
// Since: 1.0
type NotificationAction struct {
	// Label is the user-visible button text.
	Label string
	// Callback is invoked on the main goroutine when the button is activated.
	Callback func()
}

// NotificationOptions is a richer notification than [Notification]: it adds
// click activation, inline action buttons and a per-notification icon. Send it
// with [SendNotificationWithOptions] (or the [NotifierWithOptionsApp] optional
// interface on the running app).
//
// The callbacks (OnTapped and each action's Callback) are session-scoped: they
// are delivered on the main goroutine while the app that sent the notification
// is still running. Activating a notification whose app has since terminated is
// out of scope here — that requires per-platform launch activation (the "open
// with" territory of [OpenHandlerApp]).
//
// Capabilities that the current platform cannot honour degrade gracefully: an
// implementation that cannot show action buttons still shows the title, content
// and icon rather than dropping the notification. See [SendNotificationWithOptions].
//
// Since: 1.0
type NotificationOptions struct {
	// Title and Content are the notification headline and body, as in [Notification].
	Title, Content string
	// Icon optionally overrides the application icon for this one notification.
	// Nil uses the app icon.
	Icon Resource
	// OnTapped is invoked on the main goroutine when the notification body is
	// clicked (the default activation). Nil disables click handling.
	OnTapped func()
	// Actions are inline action buttons, in display order. Platforms cap the
	// number they display; excess actions are dropped from the tail.
	Actions []NotificationAction
}
