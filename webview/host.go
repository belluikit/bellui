package webview

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/widget"
)

// Host places a [WebView] in a bellui layout. Create one with [NewHost].
//
// It exists because the package spans two hosting models and an app should
// not have to know which one it got. On macOS, iOS, Android, Windows and the
// web target the engine is an OS view positioned over the canvas, hosted by a
// [widget.NativeViewHost] with all of that host's documented limitations. On
// Linux the engine (CEF) renders offscreen and the content is composited
// *inside* bellui's canvas — overlays draw above it, [core.Canvas.Capture]
// sees it, clipping and opacity apply, and the host is a Tab stop, because
// bellui owns the keyboard rather than the OS. While the page is focused,
// Tab traverses the page's own tab order; Ctrl+Tab / Ctrl+Shift+Tab return
// to bellui's focus ring (#126).
//
// All methods must be called on the main goroutine.
//
// Since: 1.0
type Host struct {
	widget.BaseWidget

	view *WebView
	impl hostImpl
}

// hostImpl is the per-model hosting widget inside a Host: a
// *widget.NativeViewHost on the hole-punch platforms, the composited
// offscreen host on Linux.
type hostImpl interface {
	focusNative() bool
	nativeFocused() bool
	setMinSize(geo.Size)
	syncStats() (syncs, applies uint64)

	// object is the CanvasObject placed in the render tree. It must be the
	// hosting widget ITSELF, never an adapter wrapping it: NativeViewHost's
	// per-frame sync finds itself in the tree by identity (CanvasForObject,
	// driver.GeometryForObject), and a wrapper in the tree means those
	// lookups compare the inner widget against the wrapper and never match —
	// the host then never attaches, on every hole-punch platform, silently.
	// #82's interactive pass caught exactly that; the renderer test locks it.
	object() core.CanvasObject
}

var _ core.Widget = (*Host)(nil)

// NewHost returns a [Host] embedding a new web view for url, and the view for
// driving it. It is the one-call form of wrapping [New] in a host.
//
// Since: 1.0
func NewHost(url string) (*Host, *WebView) {
	w := New(url)
	h := &Host{view: w, impl: newPlatformHost(w)}
	h.ExtendBaseWidget(h)
	return h, w
}

// CreateRenderer returns a renderer showing the platform hosting widget —
// the widget itself, by identity, per the hostImpl.object contract.
func (h *Host) CreateRenderer() core.WidgetRenderer {
	h.ExtendBaseWidget(h)
	return widget.NewSimpleRenderer(h.impl.object())
}

// WebView returns the view this host places.
func (h *Host) WebView() *WebView {
	return h.view
}

// SetMinSize sets the smallest size this host should be laid out at, in
// device-independent points. Web content has no intrinsic size bellui can
// measure, so the default is 0x0 and the host takes whatever the layout
// gives it.
func (h *Host) SetMinSize(min geo.Size) {
	h.impl.setMinSize(min)
	cache.MinSizeSet()
}

// FocusNativeView hands the keyboard to the web content — the OS focus on a
// hole-punch platform, canvas focus on the composited Linux host — and
// reports whether it was accepted.
func (h *Host) FocusNativeView() bool {
	return h.impl.focusNative()
}

// NativeViewFocused reports whether keystrokes currently go to the web
// content rather than to another bellui widget.
func (h *Host) NativeViewFocused() bool {
	return h.impl.nativeFocused()
}

// SyncStats reports the per-frame geometry sync counters of a hole-punch
// host — see [widget.NativeViewHost.SyncStats]. The composited Linux host
// runs no geometry sync (the content is a widget, layout reaches it like any
// other), so both counters stay zero there.
func (h *Host) SyncStats() (syncs, applies uint64) {
	return h.impl.syncStats()
}
