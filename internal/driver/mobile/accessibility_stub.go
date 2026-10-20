//go:build (!accessibility || (!android && !ios)) && !wasm

package mobile

// No-op accessibility hooks for builds without a platform screen-reader
// bridge: the accessibility tag is off, or the mobile driver is compiled in
// its host (desktop) slice.

func (w *window) updateAccessibility() {
}

func (w *window) initAccessibilityForWindow() {
}

func (w *window) cleanupAccessibilityForWindow() {
}
