//go:build (windows && !ci && !android && !ios && !wasm && !tinygo) || (!ci && !wasm && !android && !ios && !mobile && (linux || openbsd || freebsd || netbsd) && !tinygo && !noos && !tamago)

package app

import (
	"fmt"
	"os"

	"github.com/belluikit/bellui"
)

// removeTempIcon deletes a temp icon file written by writeTempNotificationIcon,
// ignoring an already-removed file.
//
// Its build constraint is the union of app_notification_windows.go's and
// app_notification_xdg.go's, its only callers: built anywhere else (macOS, the
// ci tag) it is dead code and staticcheck reports it U1000 (#474).
func removeTempIcon(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		bellui.LogError(fmt.Sprintf("Unable to remove temp notification icon %q", path), err)
	}
}
