//go:build !ci && !ios && !wasm && !mobile && !tinygo

// app_badge_darwin.go holds the macOS-only pieces split out of app_darwin.go in
// #739: the AppKit dock-tile badge and the osascript notification fallback. It
// carries the -framework AppKit link flag so the shared app_darwin.go (and the
// iOS build) need not pull AppKit in.

package app

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit

#include <stdlib.h>

void setDockBadge(char *label);
*/
import "C"

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"unsafe"

	"github.com/belluikit/bellui"
)

// SetBadge sets the dock-icon badge label to the given count, clearing it when
// count <= 0. Best-effort; part of the [bellui.BadgeApp] optional interface.
func (a *belluiApp) SetBadge(count int) {
	label := ""
	if count > 0 {
		label = strconv.Itoa(count)
	}
	labelStr := C.CString(label)
	defer C.free(unsafe.Pointer(labelStr))
	C.setDockBadge(labelStr)
}

func escapeNotificationString(in string) string {
	noSlash := strings.ReplaceAll(in, "\\", "\\\\")
	return strings.ReplaceAll(noSlash, "\"", "\\\"")
}

// fallbackNotification posts a notification via osascript, used when an
// unsigned/unbundled macOS app cannot obtain UserNotifications authorization.
func fallbackNotification(title, content string) {
	template := `display notification "%s" with title "%s"`
	script := fmt.Sprintf(template, escapeNotificationString(content), escapeNotificationString(title))

	err := exec.Command("osascript", "-e", script).Start()
	if err != nil {
		bellui.LogError("Failed to launch darwin notify script", err)
	}
}
