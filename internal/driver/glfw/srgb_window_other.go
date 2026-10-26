//go:build !darwin

package glfw

// tagSRGBWindow does nothing off macOS: the other platforms show an
// untagged surface as sRGB (#373).
func tagSRGBWindow(*window) {}
