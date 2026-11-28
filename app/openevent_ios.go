//go:build ios && !ci

package app

import (
	mobileapp "github.com/belluikit/bellui/internal/driver/mobile/app"
)

// platformLaunchOpen wires the iOS "open with" seam (pre-rebrand tracker #622) to the shared
// open-event delivery. The mobile driver reports the URL from
// application:openURL: (custom scheme or file://) or continueUserActivity: (a
// Universal Link) — buffered until now if it arrived as the launch URL — which
// classifyOpenArg splits into a file or URL open (file:// → file, http/custom
// scheme → URL). Unlike Android there is no content:// scheme on iOS.
//
// It also installs the push handlers (push_ios.go), which must be in place
// before UIKit starts, for the same reason: a tap that launches the app is
// reported during launch.
func platformLaunchOpen(a *belluiApp) {
	setupPush(a)
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
