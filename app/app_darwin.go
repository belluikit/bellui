//go:build !ci && !wasm && !mobile && !tinygo

// app_darwin.go is the shared macOS + iOS notification implementation, built on
// UNUserNotificationCenter (available on both). AppKit-only concerns (the dock
// badge, the osascript fallback) live in app_badge_darwin.{go,m} (macOS) and the
// UIKit badge in app_badge_ios.{go,m}; each platform supplies its own
// fallbackNotification. See #739.

package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework UserNotifications

#include <stdbool.h>
#include <stdlib.h>

bool isBundled();
void sendNotification(char *title, char *content);
void sendNotificationWithOptions(unsigned long long token, char *title, char *content,
	char *iconPath, char **actionLabels, int actionCount);
*/
import "C"

import (
	"unsafe"

	"github.com/belluikit/bellui"
)

func (a *belluiApp) SendNotification(n *bellui.Notification) {
	if C.isBundled() {
		titleStr := C.CString(n.Title)
		defer C.free(unsafe.Pointer(titleStr))
		contentStr := C.CString(n.Content)
		defer C.free(unsafe.Pointer(contentStr))

		C.sendNotification(titleStr, contentStr)
		return
	}

	fallbackNotification(n.Title, n.Content)
}

// SendNotificationWithOptions sends a rich notification via
// UNUserNotificationCenter, delivering the body-click and action-button
// callbacks through the shared delegate (see app_darwin.m). It requires a
// bundled app with a valid identifier; an unbundled binary (e.g. bare "go run")
// cannot use UserNotifications, so it degrades to a plain notification and the
// callbacks and actions are dropped.
func (a *belluiApp) SendNotificationWithOptions(opts *bellui.NotificationOptions) {
	if !C.isBundled() {
		a.SendNotification(plainNotification(opts))
		return
	}

	token := a.registerNotifyCallbacks(opts)

	titleStr := C.CString(opts.Title)
	defer C.free(unsafe.Pointer(titleStr))
	contentStr := C.CString(opts.Content)
	defer C.free(unsafe.Pointer(contentStr))

	iconPath := ""
	if opts.Icon != nil {
		if p, err := writeTempNotificationIcon(opts.Icon, "bellui-notify-icon-*.png"); err == nil {
			iconPath = p // owned by the system once attached; OS reaps /tmp otherwise
		} else {
			bellui.LogError("Unable to write notification icon", err)
		}
	}
	iconStr := C.CString(iconPath)
	defer C.free(unsafe.Pointer(iconStr))

	// Marshal the action labels into a C string array.
	var labels **C.char
	count := len(opts.Actions)
	if count > 0 {
		arr := C.malloc(C.size_t(count) * C.size_t(unsafe.Sizeof(uintptr(0))))
		defer C.free(arr)
		slice := unsafe.Slice((**C.char)(arr), count)
		for i, act := range opts.Actions {
			slice[i] = C.CString(act.Label)
		}
		defer func() {
			for i := range slice {
				C.free(unsafe.Pointer(slice[i]))
			}
		}()
		labels = (**C.char)(arr)
	}

	C.sendNotificationWithOptions(C.ulonglong(token), titleStr, contentStr, iconStr, labels, C.int(count))
}

// notifyCallbackDarwin is invoked from the UNUserNotificationCenter delegate when
// the user interacts with a notification. actionIndex is -1 for a body click,
// >= 0 for an action button, and any other value (e.g. a dismiss) means "no
// callback, just release the handler". It runs on the main thread; the fire*
// helpers hop the app callback onto the bellui main goroutine.
//
//export notifyCallbackDarwin
func notifyCallbackDarwin(token C.ulonglong, actionIndex C.int) {
	a, ok := bellui.CurrentApp().(*belluiApp)
	if !ok {
		return
	}
	tok := uint64(token)
	switch idx := int(actionIndex); {
	case idx == -1:
		a.fireNotifyTapped(tok)
	case idx >= 0:
		a.fireNotifyAction(tok, idx)
	}
	a.forgetNotify(tok)
}

// fallbackSend is invoked from the notification C layer when authorization
// cannot be obtained (typically an unsigned/unbundled macOS binary). It routes
// to the per-platform fallbackNotification (osascript on macOS, a no-op log on
// iOS, where apps are always bundled and this path is unreachable).
//
//export fallbackSend
func fallbackSend(cTitle, cContent *C.char) {
	title := C.GoString(cTitle)
	content := C.GoString(cContent)
	fallbackNotification(title, content)
}
