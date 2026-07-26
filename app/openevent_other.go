//go:build ci || (!wasm && !android && !ios && (mobile || tinygo || noos || tamago || !(linux || darwin || windows || freebsd || openbsd || netbsd)))

package app

// platformLaunchOpen is a no-op where there is no launch "open with" payload to
// read: the headless/test drivers (whose argv is the test binary's own flags)
// and unknown platforms. Android is handled in openevent_android.go and iOS in
// openevent_ios.go.
func platformLaunchOpen(_ *belluiApp) {}
