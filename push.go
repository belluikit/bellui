package bellui

import "errors"

// ErrPushNotSupported is reported by [RegisterForPush] and
// [RequestNotificationPermission] when the current platform cannot receive
// remote push notifications. Only iOS can today.
//
// Since: 1.7
var ErrPushNotSupported = errors.New("remote push notifications are not supported on this platform")

// PushNotification is a remote notification that the platform's push service
// (APNs on iOS) delivered to the app.
//
// Since: 1.7
type PushNotification struct {
	// Title and Body are the alert text the push carried, empty when it had
	// none.
	Title, Body string

	// Payload is the whole payload as the push service delivered it, decoded
	// from JSON. On iOS that is the APNs payload, its "aps" dictionary
	// included; an app's own keys sit beside "aps". JSON numbers decode as
	// float64.
	Payload map[string]any
}

// PushApp is an optional interface that an [App] can implement to receive
// remote push notifications: a notification an app's server sends through the
// platform's push service, which shows even when the app is closed or the
// phone is locked. It is implemented on iOS; other platforms do not satisfy
// it, and the free functions below report [ErrPushNotSupported] there.
//
// An iOS app also needs the aps-environment entitlement. Put
// $(APS_ENVIRONMENT) in ios/Entitlements.plist and the bellui command fills in
// development for `bellui package` and production for `bellui release`.
//
// Every callback runs on the main goroutine.
//
// Since: 1.7
type PushApp interface {
	// RequestNotificationPermission asks the user to allow alerts and
	// sounds, unless they have answered before, and reports the answer.
	// A denial is granted false with a nil error; err is set only when the
	// question could not be asked.
	RequestNotificationPermission(done func(granted bool, err error))

	// RegisterForPush registers with the platform's push service. onToken
	// receives the device token, as lowercase hex, the token a server sends
	// to. It is called again each time the platform issues a token: after
	// each RegisterForPush call, and when the token changes. onError
	// receives the reason when registration fails. A later call replaces
	// both callbacks.
	//
	// A token is issued whether or not the user allowed notifications; a
	// push to an app that is not allowed is not shown.
	RegisterForPush(onToken func(token string), onError func(error))

	// SetOnPushOpened sets the callback for a push the user tapped, whether
	// the tap launched the app or it was already running. A tap that
	// arrives before the callback is set is held, and delivered once it is
	// set and the app is running. A nil callback clears it; taps are then
	// held again.
	SetOnPushOpened(func(*PushNotification))

	// SetOnPushReceived sets the callback for a push that arrives while the
	// app is in the foreground. Its result decides whether the system shows
	// it anyway: true shows a banner, false shows nothing. With no callback
	// set, a foreground push is shown. Notifications the app posts itself
	// are not passed to it.
	SetOnPushReceived(func(*PushNotification) (show bool))
}

// RequestNotificationPermission asks the user to allow alerts and sounds
// through the current app (see [PushApp]). Where the app cannot, done
// receives false and [ErrPushNotSupported].
//
// Since: 1.7
func RequestNotificationPermission(done func(granted bool, err error)) {
	if p, ok := CurrentApp().(PushApp); ok {
		p.RequestNotificationPermission(done)
		return
	}
	if done != nil {
		done(false, ErrPushNotSupported)
	}
}

// RegisterForPush registers the current app with the platform's push service
// (see [PushApp]). Where the app cannot, onError receives
// [ErrPushNotSupported].
//
// Since: 1.7
func RegisterForPush(onToken func(token string), onError func(error)) {
	if p, ok := CurrentApp().(PushApp); ok {
		p.RegisterForPush(onToken, onError)
		return
	}
	if onError != nil {
		onError(ErrPushNotSupported)
	}
}

// SetOnPushOpened sets the current app's callback for a tapped push (see
// [PushApp]). It reports false when the app cannot receive pushes, so the
// callback will never run.
//
// Since: 1.7
func SetOnPushOpened(fn func(*PushNotification)) bool {
	p, ok := CurrentApp().(PushApp)
	if ok {
		p.SetOnPushOpened(fn)
	}
	return ok
}

// SetOnPushReceived sets the current app's callback for a push that arrives
// in the foreground (see [PushApp]). It reports false when the app cannot
// receive pushes, so the callback will never run.
//
// Since: 1.7
func SetOnPushReceived(fn func(*PushNotification) (show bool)) bool {
	p, ok := CurrentApp().(PushApp)
	if ok {
		p.SetOnPushReceived(fn)
	}
	return ok
}
