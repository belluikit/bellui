//go:build !ci && wasm

package app

import (
	"net/url"
	"syscall/js"

	"github.com/belluikit/bellui"
)

// platformLaunchOpen delivers the page URL as a launch "open with" URL event, so
// a web app can act on query/fragment deep links (e.g. ?open=... or #/path).
// There is no file-open equivalent on the web.
func platformLaunchOpen(a *belluiApp) {
	loc := js.Global().Get("location")
	if !loc.Truthy() {
		return
	}
	href := loc.Get("href")
	if !href.Truthy() {
		return
	}
	u, err := url.Parse(href.String())
	if err != nil {
		bellui.LogError("Failed to parse launch URL for OnURLOpened", err)
		return
	}
	a.openURL(u)
}
