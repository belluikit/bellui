//go:build !darwin && (gl || (!linux && !windows && !wasm))

package glfw

// tagSRGBWindow does nothing off macOS: the other platforms show an
// untagged surface as sRGB (#373).
func tagSRGBWindow(*window) {}
