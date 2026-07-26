//go:build !partialrepaint

package build

// PartialRepaint set to false makes the drivers full-clear and redraw the whole
// canvas into the persistent FBO every frame. This is the safe default; build
// with the partialrepaint tag to enable dirty-region partial repaints.
const PartialRepaint = false
