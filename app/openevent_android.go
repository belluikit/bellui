//go:build android && !ci

package app

import (
	"net/url"

	mobileapp "github.com/belluikit/bellui/internal/driver/mobile/app"
	"github.com/belluikit/bellui/storage"
)

// platformLaunchOpen wires the Android "open with" intent seam (pre-rebrand tracker #622) to the
// shared open-event delivery. The mobile driver reports a VIEW/EDIT data Uri
// (file association or custom URL scheme / deep link) — buffered until now if it
// arrived as the launch intent — and we classify it into a file or URL open.
func platformLaunchOpen(a *belluiApp) {
	mobileapp.SetOpenIntentHandler(a.deliverAndroidOpen)

	// Route rich-notification taps/actions (pre-rebrand tracker #723) back to the session
	// callback registry. action -1 is the body tap; >=0 is the action index.
	mobileapp.SetNotificationActivatedHandler(func(token int64, action int) {
		if action < 0 {
			a.fireNotifyTapped(uint64(token))
		} else {
			a.fireNotifyAction(uint64(token), action)
		}
	})
}

// deliverAndroidOpen classifies an Android intent Uri and hands it to the
// buffered open-event delivery. A content:// or file:// Uri (an opened document)
// is a file; an http/https or custom-scheme Uri is a URL. This differs from the
// desktop argv classifier only in treating the Android-specific content scheme
// as a file rather than a URL.
func (a *belluiApp) deliverAndroidOpen(uri string) {
	if u, err := url.Parse(uri); err == nil && (u.Scheme == "content" || u.Scheme == "file") {
		if fu, err := storage.ParseURI(uri); err == nil {
			a.openFile(fu)
		}
		return
	}

	if p, ok := classifyOpenArg(uri); ok {
		if p.link != nil {
			a.openURL(p.link)
		} else {
			a.openFile(p.uri)
		}
	}
}
