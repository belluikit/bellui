//go:build !ci && mobile && !android && !ios

package app

import (
	"errors"
	"net/url"

	"github.com/belluikit/bellui"
)

func (a *belluiApp) OpenURL(_ *url.URL) error {
	return errors.New("mobile simulator does not support open URLs yet")
}

func (a *belluiApp) SendNotification(_ *bellui.Notification) {
	bellui.LogError("Notifications are not supported in the mobile simulator yet", nil)
}

func (a *belluiApp) SendNotificationWithOptions(opts *bellui.NotificationOptions) {
	a.SendNotification(plainNotification(opts))
}

func watchTheme(_ *settings) {
	// not implemented yet
}
