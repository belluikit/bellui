//go:build windows && !amd64 && !arm64

package webview

// putBounds writes the controller's Bounds property.
//
// It is the one call in this backend whose shape depends on the architecture.
// Bounds takes a RECT *by value*, and the 32-bit stdcall convention pushes a
// struct's fields onto the stack in order rather than passing its address, so
// the rectangle travels as its four members.
func (o *comObject) putBounds(r winRect) int32 {
	return o.call(ctrlPutBounds,
		uintptr(uint32(r.left)), uintptr(uint32(r.top)),
		uintptr(uint32(r.right)), uintptr(uint32(r.bottom)))
}
