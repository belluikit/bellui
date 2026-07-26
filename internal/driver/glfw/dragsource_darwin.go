//go:build darwin && !ios && !wasm && !js

package glfw

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit
#include <stdlib.h>
#include "dragsource_darwin.h"
*/
import "C"

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
)

// startNativeDrag begins an NSDraggingSession carrying data out of the app. It is
// called on the main goroutine (from a Dragged handler), which is the AppKit main
// thread, so [NSApp currentEvent]/keyWindow resolve to the in-flight drag.
func startNativeDrag(data desktop.DragData, cursorImg image.Image) bool {
	var cText *C.char
	if data.Text != "" {
		cText = C.CString(data.Text)
		defer C.free(unsafe.Pointer(cText))
	}

	var cPaths *C.char
	if paths := filePaths(data.URIs); len(paths) > 0 {
		cPaths = C.CString(strings.Join(paths, "\n"))
		defer C.free(unsafe.Pointer(cPaths))
	}

	// cursorImg is the drag visual only (from DragData.Image or the rendered
	// ghost); it is not a data flavor.
	var imgPtr unsafe.Pointer
	var imgLen C.int
	if cursorImg != nil {
		if b := encodePNG(cursorImg); len(b) > 0 {
			imgPtr = C.CBytes(b)
			defer C.free(imgPtr)
			imgLen = C.int(len(b))
		}
	}

	// Park the completion callback for belluiDragOutCompleted: AppKit drives the
	// session asynchronously and reports the outcome through the exported C
	// callback, which cannot capture a Go closure — so unlike the goroutine-driven
	// Windows/Linux backends the callback is held here (a drag gesture is single at
	// a time). Cleared if the session fails to start.
	dragOutOnCompleted = data.OnCompleted
	if !bool(C.belluiStartDragOut(cText, cPaths, imgPtr, imgLen)) {
		dragOutOnCompleted = nil
		return false
	}
	return true
}

// dragOutOnCompleted holds the pending macOS drag-out completion callback for the
// one in-flight drag; see the note in startNativeDrag.
var dragOutOnCompleted func(accepted bool)

// belluiDragOutCompleted is called by BelluiDragSource's
// draggingSession:endedAtPoint:operation: when the drag session ends. accepted is
// non-zero when a target accepted the drop (any operation other than
// NSDragOperationNone). It delivers the parked OnCompleted callback on the main
// goroutine via bellui.Do, then clears it.
//
//export belluiDragOutCompleted
func belluiDragOutCompleted(accepted C.int) {
	cb := dragOutOnCompleted
	dragOutOnCompleted = nil
	if cb != nil {
		bellui.Do(func() { cb(accepted != 0) })
	}
}

// encodePNG renders an image to PNG bytes for the native drag image, or nil.
func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}
