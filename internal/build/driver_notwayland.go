//go:build !wayland || x11

package build

// IsWayland is true when compiling for the wayland windowing system, or when
// auto-detecting (the default build) and the Wayland backend was loaded at
// runtime. It starts false here — the default build and any forced `x11` build
// — and the driver sets it true after glfw.Init when GLFW selected Wayland
// (see loop_desktop.go). A forced `x11` build leaves it false.
var IsWayland = false
