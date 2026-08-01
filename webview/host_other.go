//go:build !linux || android || !cgo

package webview

import (
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// newPlatformHost selects the hole-punch model: the engine is an OS view
// positioned over the canvas, driven by widget.NativeViewHost. This covers
// every platform with an OS engine — and the unsupported targets, where the
// inert backend leaves the host occupying empty space, the documented
// degradation.
func newPlatformHost(w *WebView) hostImpl {
	return &nativeHostImpl{NativeViewHost: widget.NewNativeViewHost(w)}
}

// nativeHostImpl adapts widget.NativeViewHost's exported surface to the
// unexported hostImpl seam.
type nativeHostImpl struct {
	*widget.NativeViewHost
}

func (n *nativeHostImpl) focusNative() bool {
	return n.FocusNativeView()
}

func (n *nativeHostImpl) nativeFocused() bool {
	return n.NativeViewFocused()
}

func (n *nativeHostImpl) setMinSize(min geo.Size) {
	n.SetMinSize(min)
}

func (n *nativeHostImpl) syncStats() (uint64, uint64) {
	return n.SyncStats()
}

// object hands the render tree the NativeViewHost itself, not this adapter:
// the host's per-frame sync locates itself in the tree by identity, so an
// adapter standing in for it would never be matched and the platform view
// would never attach (the #82 regression).
func (n *nativeHostImpl) object() core.CanvasObject {
	return n.NativeViewHost
}
