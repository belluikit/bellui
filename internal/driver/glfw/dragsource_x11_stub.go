//go:build linux && (wayland || wasm)

package glfw

import (
	"image"

	"github.com/belluikit/bellui/driver/desktop"
)

// startDragX11 is the no-op stub for Linux builds that do not compile the X11
// backend: the forced -tags wayland single-backend build and the wasm web
// build. The default runtime-selected linux build uses the
// real cgo implementation in dragsource_x11.go; on a Wayland session
// build.IsWayland already routes around this in startNativeDrag anyway. See
// pre-rebrand tracker #838 / pre-rebrand tracker #515 Part B.
func startDragX11(desktop.DragData, image.Image) bool {
	return false
}
