//go:build !windows && (ci || wasm || android || ios || mobile || tinygo || noos || tamago || (!linux && !openbsd && !freebsd && !netbsd))

package glfw

// applyProgressNative is the no-op taskbar/launcher-progress backend for
// platforms without a native progress affordance: macOS (no dock progress),
// plus the wasm/ci/mobile builds. Windows drives ITaskbarList3 (taskbar_windows.go)
// and Linux/BSD the Unity LauncherEntry (taskbar_xdg.go).
func (w *window) applyProgressNative(value float64, indeterminate bool) {
}
