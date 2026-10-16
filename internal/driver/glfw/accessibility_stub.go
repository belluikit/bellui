//go:build !accessibility || (!darwin && !windows && (!linux || android || wasm || js))

package glfw

// Screen-reader backends are opt-in via the `accessibility` build tag and
// implemented per platform (macOS NSAccessibility pre-rebrand tracker #485, Windows UIA pre-rebrand tracker #487,
// Linux AT-SPI2 pre-rebrand tracker #981). These stubs keep untagged and not-yet-covered builds
// free of any overhead — the calls in the draw loop and window lifecycle
// compile to nothing.

func (w *window) updateAccessibility() {
}

func (w *window) cleanupAccessibilityForWindow() {
}
