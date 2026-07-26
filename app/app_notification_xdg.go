//go:build !ci && !wasm && !android && !ios && !mobile && (linux || openbsd || freebsd || netbsd) && !tinygo && !noos && !tamago

package app

import (
	"strconv"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/build"
)

const (
	dbusNotifyPath  = "/org/freedesktop/Notifications"
	dbusNotifyIface = "org.freedesktop.Notifications"

	// notifyDefaultAction is the freedesktop key for the action triggered by
	// clicking the notification body (as opposed to a labelled button).
	notifyDefaultAction = "default"
	notifyActionPrefix  = "bellui-action-"
)

var (
	notifySignalOnce sync.Once

	notifyStateMu sync.Mutex
	// notifyState maps a live D-Bus notification id to the callback token and the
	// temp icon file (if any) written for that notification, so an incoming
	// ActionInvoked/NotificationClosed signal can be routed and cleaned up.
	notifyState = map[uint32]notifyEntry{}
)

type notifyEntry struct {
	token    uint64
	iconPath string // temp file to remove on close; empty when none
}

// SendNotificationWithOptions sends a rich notification over the freedesktop
// org.freedesktop.Notifications D-Bus interface, which natively supports action
// buttons and a body-click default action, delivering both back through the
// ActionInvoked signal. Under Flatpak the sandbox portal has no callback channel
// here, so it degrades to a plain notification.
func (a *belluiApp) SendNotificationWithOptions(opts *bellui.NotificationOptions) {
	if build.IsFlatpak {
		if err := a.sendNotificationThroughPortal(plainNotification(opts)); err != nil {
			bellui.LogError("Sending notification using portal failed", err)
		}
		return
	}

	conn, err := dbus.SessionBus() // shared connection, don't close
	if err != nil {
		bellui.LogError("Unable to connect to session D-Bus", err)
		return
	}

	token := a.registerNotifyCallbacks(opts)
	a.ensureNotifySignalListener(conn)

	// actions is a flat [key, label, key, label, …] array. "default" carries no
	// visible label (it fires on body click); each button gets an indexed key.
	var actions []string
	if opts.OnTapped != nil {
		actions = append(actions, notifyDefaultAction, "")
	}
	for i, act := range opts.Actions {
		actions = append(actions, notifyActionPrefix+strconv.Itoa(i), act.Label)
	}

	icon := a.cachedIconPath()
	tempIcon := ""
	if opts.Icon != nil {
		if p, err := writeTempNotificationIcon(opts.Icon, "bellui-notify-icon-*"); err == nil {
			icon, tempIcon = p, p
		} else {
			bellui.LogError("Unable to write notification icon", err)
		}
	}

	obj := conn.Object(dbusNotifyIface, dbusNotifyPath)
	call := obj.Call(dbusNotifyIface+".Notify", 0, a.uniqueID, uint32(0),
		icon, opts.Title, opts.Content, actions, map[string]dbus.Variant{}, int32(0))
	if call.Err != nil {
		bellui.LogError("Failed to send message to bus", call.Err)
		a.forgetNotify(token)
		removeTempIcon(tempIcon)
		return
	}

	if token == 0 && tempIcon == "" {
		return // nothing to track or clean up
	}

	var id uint32
	if err := call.Store(&id); err != nil {
		bellui.LogError("Unable to read notification id", err)
		a.forgetNotify(token)
		removeTempIcon(tempIcon)
		return
	}

	notifyStateMu.Lock()
	notifyState[id] = notifyEntry{token: token, iconPath: tempIcon}
	notifyStateMu.Unlock()
}

// ensureNotifySignalListener subscribes once to the Notifications signals and
// spawns the goroutine that routes them. It is idempotent across notifications.
func (a *belluiApp) ensureNotifySignalListener(conn *dbus.Conn) {
	notifySignalOnce.Do(func() {
		err := conn.AddMatchSignal(
			dbus.WithMatchObjectPath(dbusNotifyPath),
			dbus.WithMatchInterface(dbusNotifyIface),
		)
		if err != nil {
			bellui.LogError("Unable to watch notification signals", err)
			return
		}
		ch := make(chan *dbus.Signal, 16)
		conn.Signal(ch)
		go a.consumeNotifySignals(ch)
	})
}

func (a *belluiApp) consumeNotifySignals(ch chan *dbus.Signal) {
	for sig := range ch {
		switch sig.Name {
		case dbusNotifyIface + ".ActionInvoked":
			if len(sig.Body) < 2 {
				continue
			}
			id, _ := sig.Body[0].(uint32)
			key, _ := sig.Body[1].(string)
			a.handleNotifyAction(id, key)
		case dbusNotifyIface + ".NotificationClosed":
			if len(sig.Body) < 1 {
				continue
			}
			id, _ := sig.Body[0].(uint32)
			a.handleNotifyClosed(id)
		}
	}
}

func (a *belluiApp) handleNotifyAction(id uint32, key string) {
	notifyStateMu.Lock()
	entry, ok := notifyState[id]
	notifyStateMu.Unlock()
	if !ok {
		return
	}

	if key == notifyDefaultAction {
		a.fireNotifyTapped(entry.token)
		return
	}
	if idx, ok := parseNotifyActionKey(key); ok {
		a.fireNotifyAction(entry.token, idx)
	}
}

func (a *belluiApp) handleNotifyClosed(id uint32) {
	notifyStateMu.Lock()
	entry, ok := notifyState[id]
	delete(notifyState, id)
	notifyStateMu.Unlock()
	if !ok {
		return
	}
	a.forgetNotify(entry.token)
	removeTempIcon(entry.iconPath)
}

// parseNotifyActionKey turns "bellui-action-2" back into the action index 2.
func parseNotifyActionKey(key string) (int, bool) {
	rest, ok := strings.CutPrefix(key, notifyActionPrefix)
	if !ok {
		return 0, false
	}
	idx, err := strconv.Atoi(rest)
	if err != nil {
		return 0, false
	}
	return idx, true
}
