//go:build windows && amd64

package webview

import (
	"runtime"
	"unsafe"
)

// putBounds writes the controller's Bounds property.
//
// It is the one call in this backend whose shape depends on the architecture.
// Bounds takes a RECT *by value*, and how a 16-byte struct reaches a function
// is decided by the platform ABI rather than by the API: the Windows x64
// convention passes anything larger than 8 bytes by hidden reference, so the
// argument here is the address of the struct.
func (o *comObject) putBounds(r winRect) int32 {
	hr := o.call(ctrlPutBounds, uintptr(unsafe.Pointer(&r)))
	runtime.KeepAlive(&r)
	return hr
}
