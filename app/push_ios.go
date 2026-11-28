//go:build ios && !ci

package app

import (
	"errors"
	"sync"

	"github.com/belluikit/bellui"
	mobileapp "github.com/belluikit/bellui/internal/driver/mobile/app"
)

var _ bellui.PushApp = (*belluiApp)(nil)

// pushRegistration holds the callbacks of the last RegisterForPush call. iOS
// reports a token to the app, not to a call, so one pair serves them all.
var pushRegistration struct {
	sync.Mutex
	onToken func(string)
	onError func(error)
}

// setupPush connects the iOS app delegate's reports (#1517) to a's delivery.
func setupPush(a *belluiApp) {
	mobileapp.SetPushHandlers(mobileapp.PushHandlers{
		Token: func(hex string) {
			pushRegistration.Lock()
			fn := pushRegistration.onToken
			pushRegistration.Unlock()
			if fn != nil {
				queueOnMain(func() { fn(hex) })
			}
		},
		RegisterError: func(msg string) {
			pushRegistration.Lock()
			fn := pushRegistration.onError
			pushRegistration.Unlock()
			if fn != nil {
				err := errors.New(msg)
				queueOnMain(func() { fn(err) })
			}
		},
		Opened: func(payload []byte) {
			a.pushOpened(decodePush(payload))
		},
		Received: func(payload []byte, reply func(bool)) {
			n := decodePush(payload)
			if n == nil {
				reply(true)
				return
			}
			a.pushReceived(n, reply)
		},
		LocalResponse: a.notifyResponse,
	})
}

// RequestNotificationPermission implements bellui.PushApp.
func (a *belluiApp) RequestNotificationPermission(done func(granted bool, err error)) {
	mobileapp.RequestNotificationPermission(func(granted bool, errMsg string) {
		if done == nil {
			return
		}
		var err error
		if errMsg != "" {
			err = errors.New(errMsg)
		}
		queueOnMain(func() { done(granted, err) })
	})
}

// RegisterForPush implements bellui.PushApp.
func (a *belluiApp) RegisterForPush(onToken func(token string), onError func(error)) {
	pushRegistration.Lock()
	pushRegistration.onToken = onToken
	pushRegistration.onError = onError
	pushRegistration.Unlock()
	mobileapp.RegisterForPush()
}

// SetOnPushOpened implements bellui.PushApp.
func (a *belluiApp) SetOnPushOpened(fn func(*bellui.PushNotification)) {
	a.setOnPushOpened(fn)
}

// SetOnPushReceived implements bellui.PushApp.
func (a *belluiApp) SetOnPushReceived(fn func(*bellui.PushNotification) bool) {
	a.setOnPushReceived(fn)
}
