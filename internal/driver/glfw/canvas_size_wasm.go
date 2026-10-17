//go:build wasm

package glfw

// canvasFixedSize: in the browser the window is the page's viewport and cannot
// grow, so content wider or taller than it is clipped rather than growing the
// canvas (see common.Canvas.FixedSize).
const canvasFixedSize = true
