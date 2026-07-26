//go:build !ci && !android && !ios && !mobile && !wasm && !tinygo && !noos && !tamago && (linux || windows || freebsd || openbsd || netbsd)

package app

import "os"

// platformLaunchOpen delivers the launch "open with" payload on non-macOS
// desktop: OS file associations and custom URL schemes pass the target in the
// command line on a fresh launch. (macOS uses Apple Events instead - see
// openevent_darwin.go.)
func platformLaunchOpen(a *belluiApp) {
	a.dispatchOpenArgs(os.Args[1:])
}
