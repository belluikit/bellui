//go:build !gl && (darwin || linux || windows) && !wasm

package mmedia

// streamingSupported mirrors bellui's painter selection. This build uses the
// WebGPU painter, which is the default on desktop and implements no streaming
// path for canvas.Raster.
//
// Setting Raster.Streaming here would not merely fail to help — it would break
// rendering outright. Canvas.freeObject skips releasing the texture of a
// streaming raster, on the assumption that the painter updates it in place;
// WebGPU never does, so the first texture would be cached forever and the
// video would stay frozen on its first frame.
const streamingSupported = false
