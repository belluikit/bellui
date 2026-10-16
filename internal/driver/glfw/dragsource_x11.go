//go:build (linux && !wayland && !wasm) || freebsd || openbsd || netbsd

package glfw

/*
#cgo pkg-config: x11 xext
#include <stdlib.h>
#include "dragsource_x11.h"
*/
import "C"

import (
	"image"
	"runtime"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
)

// startDragX11 begins a hand-rolled XDND source drag out of the app (pre-rebrand tracker #838,
// pre-rebrand tracker #515 Part B). GLFW exposes no drag-source API, so the C side (dragsource_x11.c)
// drives the protocol on a private Xlib connection: it owns the XdndSelection,
// polls the pointer, negotiates with the window under it and serves the payload.
// The whole drag runs on a dedicated locked OS thread with its own Display
// connection, so it never touches GLFW's connection or event loop (no Xlib
// cross-thread hazard). It returns false only when X is unreachable, so the
// caller keeps its in-app drag behaviour; otherwise it returns true immediately
// and reports the drop outcome via notifyDragOutCompleted when the drag ends.
// cursorImg, when set, is drawn in a click-through window that trails the pointer
// for the whole drag (pre-rebrand tracker #865) — the visual feedback plain XDND lacks; it carries no
// data flavor (the image *data* flavor still comes from an explicit
// DragData.Image in buildDragFlavors).
func startDragX11(data desktop.DragData, cursorImg image.Image) bool {
	flavors := buildDragFlavors(data)
	if len(flavors) == 0 {
		return false
	}
	// Honest capability probe on the calling goroutine: decline (keep in-app
	// behaviour) rather than silently swallow the gesture when X is unreachable.
	if C.belluiX11Available() == 0 {
		return false
	}

	// Capture the completion callback in the drag goroutine (the completion is
	// reported synchronously when belluiStartDragX11 returns), so no shared package
	// state is needed — see the note in dragsource.go's StartDragging.
	onCompleted := data.OnCompleted

	// Convert the drag-icon image up-front (main goroutine); the bytes are copied
	// into C memory inside the drag goroutine and handed to the C poll loop.
	iconBGRA, iconW, iconH := imageToBGRAPremul(cursorImg)

	go func() {
		// Pin to one OS thread: the private Xlib Display is used only by this
		// goroutine, so single-threaded access is safe without XInitThreads.
		runtime.LockOSThread()

		// Allocate the flavor array and its payloads in C memory; the C call
		// blocks for the whole drag, so they must outlive it (freed after).
		cArr := C.malloc(C.size_t(len(flavors)) * C.size_t(unsafe.Sizeof(C.BelluiDragFlavor{})))
		arr := unsafe.Slice((*C.BelluiDragFlavor)(cArr), len(flavors))
		for i, f := range flavors {
			arr[i].mime = C.CString(f.mime)
			arr[i].data = C.CBytes(f.data)
			arr[i].len = C.int(len(f.data))
		}

		var cIcon unsafe.Pointer
		if len(iconBGRA) > 0 {
			cIcon = C.CBytes(iconBGRA)
		}

		accepted := C.belluiStartDragX11((*C.BelluiDragFlavor)(cArr), C.int(len(flavors)),
			(*C.uchar)(cIcon), C.int(iconW), C.int(iconH)) != 0

		if cIcon != nil {
			C.free(cIcon)
		}
		for i := range arr {
			C.free(unsafe.Pointer(arr[i].mime))
			C.free(unsafe.Pointer(arr[i].data))
		}
		C.free(cArr)

		// Deliver the outcome on the main goroutine (bellui.Do), matching the other
		// backends; the callback is single-shot per drag.
		if onCompleted != nil {
			bellui.Do(func() { onCompleted(accepted) })
		}
	}()

	return true
}

// imageToBGRAPremul flattens img into premultiplied BGRA bytes (one 32-bit
// little-endian 0xAARRGGBB word per pixel, row major, top-down) — the layout the
// C drag-icon window's ARGB32 XImage expects (see dragsource_x11.h). color.RGBA()
// already returns alpha-premultiplied 16-bit components, so the high byte of each
// is the premultiplied 8-bit value. Returns nil,0,0 when img is nil/empty.
func imageToBGRAPremul(img image.Image) ([]byte, int, int) {
	if img == nil {
		return nil, 0, 0
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, 0, 0
	}
	out := make([]byte, w*h*4)
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			out[i+0] = byte(bl >> 8) // B
			out[i+1] = byte(g >> 8)  // G
			out[i+2] = byte(r >> 8)  // R
			out[i+3] = byte(a >> 8)  // A
			i += 4
		}
	}
	return out, w, h
}
