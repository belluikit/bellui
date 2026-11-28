package app

import (
	"encoding/json"

	"github.com/belluikit/bellui"
)

// Remote push delivery (#1517), shared by every build so the rules are tested
// headless; only iOS (push_ios.go) exports them as bellui.PushApp and feeds
// them from the platform.
//
// A tapped push rides the open-event buffer (openevent.go): it is held until
// the app is running and SetOnPushOpened has a callback, exactly as an opened
// URL is, because a tap that launches the app arrives before the app's code
// has set anything. A push that arrives in the foreground is not held: its
// presentation is decided now, so with no callback it is simply shown.

// setOnPushOpened stores the tapped-push callback and delivers what was held
// for it.
func (a *belluiApp) setOnPushOpened(fn func(*bellui.PushNotification)) {
	a.openMu.Lock()
	a.onPushOpened = fn
	a.flushOpensLocked()
	a.openMu.Unlock()
}

// setOnPushReceived stores the foreground-push callback.
func (a *belluiApp) setOnPushReceived(fn func(*bellui.PushNotification) bool) {
	a.pushMu.Lock()
	a.onPushReceived = fn
	a.pushMu.Unlock()
}

// pushOpened delivers a tapped push, or holds it.
func (a *belluiApp) pushOpened(n *bellui.PushNotification) {
	if n == nil {
		return
	}
	a.deliverOpen(openPayload{push: n})
}

// pushReceived asks the foreground-push callback whether to show n, and
// answers through reply exactly once: at once (true) when no callback is set,
// otherwise from the main goroutine with the callback's result.
func (a *belluiApp) pushReceived(n *bellui.PushNotification, reply func(show bool)) {
	a.pushMu.Lock()
	fn := a.onPushReceived
	a.pushMu.Unlock()
	if fn == nil {
		reply(true)
		return
	}
	queueOnMain(func() { reply(fn(n)) })
}

// decodePush turns a push's payload, as JSON, into a PushNotification. The
// alert text is read from aps.alert, which APNs allows as either a string
// (the body) or a dictionary with title and body. A payload that is not a
// JSON object yields nil.
func decodePush(payload []byte) *bellui.PushNotification {
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil || m == nil {
		return nil
	}
	n := &bellui.PushNotification{Payload: m}
	aps, _ := m["aps"].(map[string]any)
	switch alert := aps["alert"].(type) {
	case string:
		n.Body = alert
	case map[string]any:
		n.Title, _ = alert["title"].(string)
		n.Body, _ = alert["body"].(string)
	}
	return n
}
