//go:build wasm || js

package driver

import "syscall/js"

// BrowserWindowContext is passed to the NativeWindow.RunNative callback when
// it is executed for a browser-hosted window on the web driver. Canvas is the
// HTML <canvas> element rendering the window's content.
//
// Beyond handing out the canvas, the context is the web driver's window
// identity: the wasm file-dialog overrides assert NativeWindow on the dialog's
// parent (#257), so a window that cannot produce it — the test driver's, or an
// emulated extra window — is served by the canvas dialog instead of a real
// browser picker.
//
// Since: 2.10
type BrowserWindowContext struct {
	// Canvas is the <canvas> element hosting the window, as a js.Value. It is
	// the zero js.Value when the window has no canvas yet (not shown).
	Canvas js.Value
}
