//go:build partialrepaint

package build

// PartialRepaint set to true enables dirty-region partial repaints: each frame
// clears and redraws only the region covered by refreshed/moved objects in the
// persistent canvas FBO, culling everything else. It is opt-in because the GL
// paint path cannot be exercised by the (software-painter) golden test suite,
// so correctness must be validated on-device. Without the tag the drivers do a
// full clear and redraw every frame (the safe default).
const PartialRepaint = true
