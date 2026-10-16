//go:build (linux && (x11 || wasm)) || freebsd || openbsd || netbsd

package glfw

import "github.com/belluikit/bellui/driver/desktop"

// startDragWayland is the no-op stub for linux builds without the Wayland
// backend: the forced -tags x11 single-backend build and the wasm web
// build. The default runtime-selected build and the forced
// -tags wayland build use the real dragsource_wayland.go; there build.IsWayland
// routes between it and the X11 backend in startNativeDrag. See pre-rebrand tracker #838 / pre-rebrand tracker #853.
func startDragWayland(desktop.DragData) bool {
	return false
}
