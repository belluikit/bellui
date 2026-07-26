//go:build wayland && !x11

package build

// IsWayland is true when compiling for the wayland windowing system, or when
// auto-detecting (the default build) and the Wayland backend was loaded at
// runtime. In a forced `wayland` build it is always true; in the default build
// it starts false and is set by the driver after glfw.Init (see loop_desktop.go).
var IsWayland = true
