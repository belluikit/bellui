//go:build ci || (!ios && !android && !linux && !darwin && !windows && !freebsd && !openbsd && !netbsd && !wasm) || tamago || noos || tinygo

package app

import (
	"errors"
	"net/url"

	"github.com/belluikit/bellui"
)

func (a *belluiApp) OpenURL(_ *url.URL) error {
	return errors.New("unable to open url for unknown operating system")
}

func (a *belluiApp) SendNotification(_ *bellui.Notification) {
	bellui.LogError("Refusing to show notification for unknown operating system", nil)
}

func (a *belluiApp) SendNotificationWithOptions(opts *bellui.NotificationOptions) {
	a.SendNotification(plainNotification(opts))
}

func watchTheme(_ *settings) {
	// no-op
}

func (a *belluiApp) registerRepositories() {
	// no-op
}
