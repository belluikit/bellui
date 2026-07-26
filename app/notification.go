package app

import (
	"fmt"
	"os"

	"github.com/belluikit/bellui"
)

// notifyCallbacks holds the session-scoped callbacks for one rich notification,
// keyed by a bellui-generated token that each platform maps to its native
// notification handle.
type notifyCallbacks struct {
	onTapped func()
	actions  []func() // indexed by action position
}

// registerNotifyCallbacks stores the callbacks for opts and returns a token the
// platform layer threads through to its native notification so that an incoming
// activation can be routed back. It returns 0 when there is nothing to track
// (no click handler and no action callbacks), letting the platform skip the
// activation plumbing entirely.
func (a *belluiApp) registerNotifyCallbacks(opts *bellui.NotificationOptions) uint64 {
	if opts.OnTapped == nil && len(opts.Actions) == 0 {
		return 0
	}

	cb := &notifyCallbacks{onTapped: opts.OnTapped}
	for _, act := range opts.Actions {
		cb.actions = append(cb.actions, act.Callback)
	}

	a.notifyMu.Lock()
	defer a.notifyMu.Unlock()
	a.notifyCounter++
	token := a.notifyCounter
	if a.notifyHandlers == nil {
		a.notifyHandlers = make(map[uint64]*notifyCallbacks)
	}
	a.notifyHandlers[token] = cb
	return token
}

// fireNotifyTapped delivers the click callback for token on the main goroutine.
// Unknown tokens and notifications without a click handler are ignored.
func (a *belluiApp) fireNotifyTapped(token uint64) {
	a.notifyMu.Lock()
	cb := a.notifyHandlers[token]
	a.notifyMu.Unlock()
	if cb != nil && cb.onTapped != nil {
		queueOnMain(cb.onTapped)
	}
}

// fireNotifyAction delivers the action callback at index for token on the main
// goroutine. Out-of-range indices and unknown tokens are ignored.
func (a *belluiApp) fireNotifyAction(token uint64, index int) {
	a.notifyMu.Lock()
	cb := a.notifyHandlers[token]
	a.notifyMu.Unlock()
	if cb == nil || index < 0 || index >= len(cb.actions) || cb.actions[index] == nil {
		return
	}
	queueOnMain(cb.actions[index])
}

// forgetNotify drops the callbacks for token once the notification is closed or
// resolved, so a long-running app does not accumulate stale handlers.
func (a *belluiApp) forgetNotify(token uint64) {
	if token == 0 {
		return
	}
	a.notifyMu.Lock()
	delete(a.notifyHandlers, token)
	a.notifyMu.Unlock()
}

// plainNotification reduces rich options to the basic notification form, used by
// platforms (and build targets) that cannot honour click activation or actions.
func plainNotification(opts *bellui.NotificationOptions) *bellui.Notification {
	return &bellui.Notification{Title: opts.Title, Content: opts.Content}
}

// writeTempNotificationIcon writes a per-notification icon resource to a temp
// file, returning its path. The OS notification daemon reads the file when
// displaying the notification. pattern follows os.CreateTemp (a "*" is replaced
// by a random string, any suffix after it is kept, e.g. "bellui-notify-*.png").
func writeTempNotificationIcon(res bellui.Resource, pattern string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	name := f.Name()
	if _, err := f.Write(res.Content()); err != nil {
		f.Close()
		os.Remove(name)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

// removeTempIcon deletes a temp icon file written by writeTempNotificationIcon,
// ignoring an already-removed file.
func removeTempIcon(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		bellui.LogError(fmt.Sprintf("Unable to remove temp notification icon %q", path), err)
	}
}
