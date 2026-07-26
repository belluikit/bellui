//go:build (!linux || (x11 && !wayland) || (!x11 && wayland)) && !wasm

package glfw

// forcePlatform returns platformAuto when either x11 or wayland was forced (a
// single-backend build cannot switch platforms), or when we are not on a linux
// host with both backends compiled.
func forcePlatform() string {
	return platformAuto
}
