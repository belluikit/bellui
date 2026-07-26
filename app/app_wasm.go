//go:build !ci && (!android || !ios || !mobile) && wasm

package app

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"syscall/js"
	"time"

	"github.com/belluikit/bellui"
	intRepo "github.com/belluikit/bellui/internal/repository"
	"github.com/belluikit/bellui/storage/repository"
)

func (a *belluiApp) SendNotification(n *bellui.Notification) {
	notification := js.Global().Get("window").Get("Notification")
	if notification.IsUndefined() {
		bellui.LogError("Current browser does not support notifications.", nil)
		return
	}

	permission := notification.Get("permission")
	if permission.Type() != js.TypeString || permission.String() != "granted" {
		request := js.FuncOf(func(this js.Value, args []js.Value) any {
			if len(args) > 0 && args[0].Type() == js.TypeString && args[0].String() == "granted" {
				a.showNotification(n, &notification)
			} else {
				bellui.LogError("User rejected the request for notifications.", nil)
			}
			return nil
		})
		defer request.Release()
		notification.Call("requestPermission", request)
		return
	}

	a.showNotification(n, &notification)
}

func (a *belluiApp) showNotification(data *bellui.Notification, notification *js.Value) {
	a.showNotificationJS(data.Title, data.Content, nil, nil)
}

// SendNotificationWithOptions honours the click callback and per-notification
// icon via the browser Notification API. Action buttons additionally need a
// Service Worker (the page-context Notification cannot render them): when the
// options carry actions and a worker is available the notification is routed
// through registration.showNotification and every click is delivered back via
// the worker. Without actions — or when no worker can be registered — it falls
// back to the basic Notification path (body click still works, actions dropped).
// The notification itself is always shown.
func (a *belluiApp) SendNotificationWithOptions(opts *bellui.NotificationOptions) {
	notification := js.Global().Get("window").Get("Notification")
	if notification.IsUndefined() {
		bellui.LogError("Current browser does not support notifications.", nil)
		return
	}

	deliver := func() {
		if len(opts.Actions) > 0 && !js.Global().Get("navigator").Get("serviceWorker").IsUndefined() {
			a.sendNotificationViaServiceWorker(opts)
			return
		}
		a.showNotificationJS(opts.Title, opts.Content, opts.Icon, opts.OnTapped)
	}

	permission := notification.Get("permission")
	if permission.Type() != js.TypeString || permission.String() != "granted" {
		var request js.Func
		request = js.FuncOf(func(this js.Value, args []js.Value) any {
			defer request.Release()
			if len(args) > 0 && args[0].Type() == js.TypeString && args[0].String() == "granted" {
				deliver()
			} else {
				bellui.LogError("User rejected the request for notifications.", nil)
			}
			return nil
		})
		notification.Call("requestPermission", request)
		return
	}

	deliver()
}

// showNotificationJS creates a page-context browser Notification, using iconRes
// (or the app icon when nil) and wiring onclick to onTapped when set. The onclick
// closure is released when the notification is closed so it does not leak.
func (a *belluiApp) showNotificationJS(title, body string, iconRes bellui.Resource, onTapped func()) {
	notification := js.Global().Get("window").Get("Notification")

	obj := notification.New(title, map[string]any{
		"body": body,
		"icon": a.notificationIconDataURL(iconRes),
	})

	if onTapped == nil {
		return
	}

	var onClick, onClose js.Func
	onClick = js.FuncOf(func(this js.Value, args []js.Value) any {
		queueOnMain(onTapped)
		return nil
	})
	onClose = js.FuncOf(func(this js.Value, args []js.Value) any {
		onClick.Release()
		onClose.Release()
		return nil
	})
	obj.Set("onclick", onClick)
	obj.Set("onclose", onClose)
}

// notificationIconDataURL encodes iconRes (or the app icon when nil) as a data:
// URL for a Notification icon, or "" when neither is available.
func (a *belluiApp) notificationIconDataURL(iconRes bellui.Resource) string {
	res := iconRes
	if res == nil {
		res = a.icon
	}
	if res == nil {
		return ""
	}
	icon := res.Content()
	if len(icon) == 0 {
		return ""
	}
	base64Str := base64.StdEncoding.EncodeToString(icon)
	return fmt.Sprintf("data:%s;base64,%s", http.DetectContentType(icon), base64Str)
}

// ---- Service Worker rich-notification path (#724) --------------------------

// notifyActionPrefixWasm labels a showNotification action so its click can be
// mapped back to the bellui action index.
const notifyActionPrefixWasm = "bellui-action-"

// notifySWLifetime bounds how long a rich notification's callbacks are retained
// when no click arrives (the page gets no reliable dismissed signal for a
// service-worker notification), mirroring the Windows toast backstop.
const notifySWLifetime = 10 * time.Minute

var (
	swNotifyMu      sync.Mutex
	swNotifyState   int // 0 = unstarted, 1 = registering, 2 = resolved
	swNotifyReg     js.Value
	swNotifyOK      bool
	swNotifyWaiters []func(reg js.Value, ok bool)
	swNotifyMsgFn   js.Func
)

// sendNotificationViaServiceWorker registers the callbacks, ensures the worker is
// active and shows the notification with its action buttons, falling back to the
// basic Notification path if the worker cannot be registered.
func (a *belluiApp) sendNotificationViaServiceWorker(opts *bellui.NotificationOptions) {
	token := a.registerNotifyCallbacks(opts)
	iconURL := a.notificationIconDataURL(opts.Icon)

	a.ensureNotificationServiceWorker(func(reg js.Value, ok bool) {
		if !ok {
			// No worker: show a basic notification (body click still routed via
			// its own onclick) and drop the unroutable action callbacks.
			bellui.LogError("Notification actions require a Service Worker; showing a basic notification instead", nil)
			a.showNotificationJS(opts.Title, opts.Content, opts.Icon, opts.OnTapped)
			a.forgetNotify(token)
			return
		}

		actions := make([]any, 0, len(opts.Actions))
		for i, act := range opts.Actions {
			actions = append(actions, map[string]any{
				"action": notifyActionPrefixWasm + strconv.Itoa(i),
				"title":  act.Label,
			})
		}
		reg.Call("showNotification", opts.Title, map[string]any{
			"body":    opts.Content,
			"icon":    iconURL,
			"actions": actions,
			"data":    map[string]any{"belluiNotify": true, "token": float64(token)},
		})

		if token != 0 {
			time.AfterFunc(notifySWLifetime, func() { a.forgetNotify(token) })
		}
	})
}

// ensureNotificationServiceWorker lazily registers bellui-sw.js and invokes cb with
// the active registration (ok=true) or reports failure (ok=false). Concurrent
// callers before the registration resolves are queued and all notified once.
func (a *belluiApp) ensureNotificationServiceWorker(cb func(reg js.Value, ok bool)) {
	container := js.Global().Get("navigator").Get("serviceWorker")
	if container.IsUndefined() {
		cb(js.Value{}, false)
		return
	}

	swNotifyMu.Lock()
	switch swNotifyState {
	case 2: // resolved
		reg, ok := swNotifyReg, swNotifyOK
		swNotifyMu.Unlock()
		cb(reg, ok)
		return
	case 1: // registering
		swNotifyWaiters = append(swNotifyWaiters, cb)
		swNotifyMu.Unlock()
		return
	}
	swNotifyState = 1
	swNotifyWaiters = append(swNotifyWaiters, cb)
	swNotifyMu.Unlock()

	a.installServiceWorkerMessageListener(container)

	finish := func(reg js.Value, ok bool) {
		swNotifyMu.Lock()
		swNotifyState, swNotifyReg, swNotifyOK = 2, reg, ok
		waiters := swNotifyWaiters
		swNotifyWaiters = nil
		swNotifyMu.Unlock()
		for _, w := range waiters {
			w(reg, ok)
		}
	}

	var okFn, readyFn, errFn js.Func
	okFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		defer okFn.Release()
		// Wait for an active, controlling worker before using showNotification.
		readyFn = js.FuncOf(func(this js.Value, readyArgs []js.Value) any {
			defer readyFn.Release()
			reg := js.Value{}
			if len(readyArgs) > 0 {
				reg = readyArgs[0]
			}
			finish(reg, reg.Truthy())
			return nil
		})
		container.Get("ready").Call("then", readyFn)
		return nil
	})
	errFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		defer errFn.Release()
		bellui.LogError("Failed to register notification service worker", nil)
		finish(js.Value{}, false)
		return nil
	})
	container.Call("register", "bellui-sw.js").Call("then", okFn).Call("catch", errFn)
}

// installServiceWorkerMessageListener wires the page to receive click messages
// forwarded by bellui-sw.js and route them to the notify registry.
func (a *belluiApp) installServiceWorkerMessageListener(container js.Value) {
	swNotifyMsgFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		data := args[0].Get("data")
		if data.Type() != js.TypeObject || data.Get("belluiNotify").Type() != js.TypeBoolean || !data.Get("belluiNotify").Bool() {
			return nil
		}
		tokenVal := data.Get("token")
		if tokenVal.Type() != js.TypeNumber {
			return nil
		}
		action := ""
		if act := data.Get("action"); act.Type() == js.TypeString {
			action = act.String()
		}
		a.routeServiceWorkerNotify(uint64(tokenVal.Float()), action)
		return nil
	})
	container.Call("addEventListener", "message", swNotifyMsgFn)
}

// routeServiceWorkerNotify fires the body-click or action callback for a click
// forwarded by the service worker, then forgets the (now consumed) callbacks.
func (a *belluiApp) routeServiceWorkerNotify(token uint64, action string) {
	if action == "" {
		a.fireNotifyTapped(token)
	} else if rest, ok := strings.CutPrefix(action, notifyActionPrefixWasm); ok {
		if idx, err := strconv.Atoi(rest); err == nil {
			a.fireNotifyAction(token, idx)
		}
	}
	a.forgetNotify(token)
}

var themeChanged = js.FuncOf(func(this js.Value, args []js.Value) any {
	if len(args) > 0 && args[0].Type() == js.TypeObject {
		bellui.CurrentApp().Settings().(*settings).setupTheme()
	}
	return nil
})

func watchTheme(_ *settings) {
	js.Global().Call("matchMedia", "(prefers-color-scheme: dark)").Call("addEventListener", "change", themeChanged)
}

func stopWatchingTheme() {
	js.Global().Call("matchMedia", "(prefers-color-scheme: dark)").Call("removeEventListener", "change", themeChanged)
}

func (a *belluiApp) registerRepositories() {
	repo, err := intRepo.NewIndexDBRepository()
	if err != nil {
		bellui.LogError("failed to create repository: %v", err)
		return
	}

	repository.Register("idbfile", repo)
}
