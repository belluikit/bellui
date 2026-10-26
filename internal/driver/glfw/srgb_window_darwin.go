//go:build darwin

package glfw

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit

#import <AppKit/AppKit.h>

// setWindowSRGB declares the window's contents sRGB, so the window server
// colour-matches them to the display. Without it, an OpenGL view's pixels go
// to a wide-gamut display unconverted: measured on a MacBook Pro's P3 screen,
// sRGB red showed as full P3 red (#373).
static void setWindowSRGB(void *nsWindow) {
	[(__bridge NSWindow *)nsWindow setColorSpace:[NSColorSpace sRGBColorSpace]];
}
*/
import "C"

import "unsafe"

// tagSRGBWindow declares w's NSWindow sRGB (#373). The GL path calls it; the
// WebGPU path tags its Metal layer instead.
func tagSRGBWindow(w *window) {
	if w.viewport == nil {
		return
	}
	C.setWindowSRGB(unsafe.Pointer(w.viewport.GetCocoaWindow()))
}
