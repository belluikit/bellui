//go:build !wasm && !darwin && !ios && !android && !windows

package webview

import (
	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/geo"
)

// Supported reports whether this platform has a web engine binding. macOS,
// iOS, Android, Windows and the web target bind one; this file covers
// everything else — most importantly Linux, where the decision is that there
// will not be one (#60: the embeddable engine is LGPL, outside the allowed
// dependency set, and Wayland has no cross-toolkit embedding primitive).
//
// A target here reports false and leaves the host inert rather than failing to
// compile, so a cross-platform app builds everywhere and degrades to an empty
// space.
//
// Since: 2.19
const Supported = false

// webView carries no platform view on an unsupported target, only the URL it
// was asked for — which is state the shared WebView reads back through URL().
type webView struct {
	url string
}

func (w *WebView) init() {}

// attachRaw reports that this platform has no web engine binding. The whole
// public surface stays callable: bridge and eval calls report
// [ErrNotAttached] through their callbacks rather than hanging, and the
// fire-and-forget ones do nothing.
func (w *WebView) attachRaw(bellui.Window) error {
	return ErrNotAttached
}

func (w *WebView) destroyRaw() {}

func (w *WebView) attachedRaw() bool { return false }

func (w *WebView) evalRaw(string) {}

func (w *WebView) evalResultRaw(string, uint64) {}

func (w *WebView) loadURLRaw(string) {}

func (w *WebView) loadHTMLRaw(string, string) {}

func (w *WebView) navRaw(navOp) bool { return false }

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
