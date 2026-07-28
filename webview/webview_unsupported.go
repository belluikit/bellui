//go:build !(darwin && !ios && !wasm)

package webview

import (
	"errors"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
)

// Supported reports whether this platform has a web engine binding. The
// spike (#45) binds macOS only; every other target reports false and leaves
// the host inert rather than failing to compile, so a cross-platform app
// builds everywhere and degrades to an empty space.
//
// Since: 2.19
const Supported = false

// webView carries no platform view on an unsupported target, only the URL it
// was asked for — which is state the shared WebView reads back through URL().
type webView struct {
	url string
}

func (w *WebView) init() {}

// Attach reports that this platform has no web engine binding.
func (w *WebView) Attach(bellui.Window) error {
	return errors.New("webview: not supported on this platform")
}

// LoadURL records the URL; nothing loads it on this platform.
func (w *WebView) LoadURL(url string) { w.url = url }

// SetGeometry is a no-op without a platform view.
func (w *WebView) SetGeometry(rect, clip geo.Rectangle, scale float32) {}

// SetVisible is a no-op without a platform view.
func (w *WebView) SetVisible(visible bool) {}

// TakeFocus cannot give focus to an engine that does not exist.
func (w *WebView) TakeFocus() bool { return false }

// ResignFocus has no focus to give back; it reports success so a host's focus
// bridge treats it as already resigned.
func (w *WebView) ResignFocus() bool { return true }

// HasFocus is always false without a platform view.
func (w *WebView) HasFocus() bool { return false }

// Eval is a no-op without a platform view.
func (w *WebView) Eval(js string) {}

// Destroy is a no-op without a platform view.
func (w *WebView) Destroy() {}
