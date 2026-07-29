//go:build gl || (!darwin && !linux && !windows && !wasm)

package mmedia

// streamingSupported mirrors bellui's painter selection. This build uses the GL
// painter (internal/driver/glfw/backend_default.go carries the same
// constraint), which implements canvas.Raster streaming: SetPixels updates the
// existing texture in place via TexSubImage2D.
const streamingSupported = true
