//go:build windows && arm64

package webview

import "unsafe"

// putBounds writes the controller's Bounds property.
//
// It is the one call in this backend whose shape depends on the architecture.
// Bounds takes a RECT *by value*, and the Windows ARM64 convention puts a
// struct of at most 16 bytes in registers rather than passing its address — so
// the four 32-bit fields travel as the two register-sized words they pack into,
// and passing a pointer here (as x64 requires) would have the engine read the
// pointer itself as the rectangle.
func (o *comObject) putBounds(r winRect) int32 {
	words := (*[2]uintptr)(unsafe.Pointer(&r))
	return o.call(ctrlPutBounds, words[0], words[1])
}
