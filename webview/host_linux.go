//go:build linux && !android && cgo

package webview

import "github.com/belluikit/bellui/geo"

// newPlatformHost selects the composited offscreen host: on Linux the engine
// renders offscreen and the content lives inside bellui's canvas, not in an
// OS view above it. See host_offscreen_linux.go.
func newPlatformHost(w *WebView) hostImpl {
	return newOffscreenHost(w)
}

func (h *offscreenHost) focusNative() bool {
	return h.focusSelf()
}

func (h *offscreenHost) nativeFocused() bool {
	return h.focused
}

func (h *offscreenHost) setMinSize(min geo.Size) {
	h.min = min
}

// syncStats reports zeros: the composited host needs no per-frame geometry
// sync — the content is a widget, so layout, scrolling and clipping reach it
// the ordinary way.
func (h *offscreenHost) syncStats() (uint64, uint64) {
	return 0, 0
}
