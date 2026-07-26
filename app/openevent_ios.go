//go:build ios && !ci

package app

import (
	mobileapp "github.com/belluikit/bellui/internal/driver/mobile/app"
)

// platformLaunchOpen wires the iOS "open with" seam (fyne#622) to the shared
// open-event delivery. The mobile driver reports the URL from
// application:openURL: (custom scheme or file://) or continueUserActivity: (a
// Universal Link) — buffered until now if it arrived as the launch URL — which
// classifyOpenArg splits into a file or URL open (file:// → file, http/custom
// scheme → URL). Unlike Android there is no content:// scheme on iOS.
func platformLaunchOpen(a *belluiApp) {
	mobileapp.SetOpenIntentHandler(func(uri string) {
		p, ok := classifyOpenArg(uri)
		if !ok {
			return
		}
		if p.link != nil {
			a.openURL(p.link)
		} else {
			a.openFile(p.uri)
		}
	})
}
