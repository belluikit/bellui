//go:build windows && !wasm

package glfw

/*
#cgo LDFLAGS: -luser32 -lgdi32 -lole32 -lshell32
#include <stdlib.h>
#include "clipboard_flavors_windows.h"
*/
import "C"

import (
	"bytes"
	"image"
	"image/png"
	"time"
	"unsafe"

	"github.com/belluikit/bellui"
	internalclipboard "github.com/belluikit/bellui/internal/clipboard"
	"github.com/belluikit/bellui/storage"
)

// Declare conformity with the optional clipboard flavor interfaces (#513).
var (
	_ bellui.ImageClipboard       = clipboard{}
	_ bellui.FileClipboard        = clipboard{}
	_ bellui.HTMLClipboard        = clipboard{}
	_ bellui.MultiFlavorClipboard = clipboard{}
)

// withClipboardRetry runs op up to 3 times while it reports that opening the
// clipboard failed — the classic transient "Access Denied" when another
// process holds it (PR#1679). Absent flavors are not retried.
func withClipboardRetry(op func() C.int) C.int {
	status := op()
	for i := 0; i < 2 && status == C.BELLUI_CB_ERR_OPEN; i++ {
		time.Sleep(50 * time.Millisecond)
		status = op()
	}
	return status
}

func logClipboardFailure(action string, status C.int) {
	switch status {
	case C.BELLUI_CB_ERR_OPEN:
		bellui.LogError("Failed to open the clipboard to "+action, nil)
	case C.BELLUI_CB_ERR_OTHER:
		bellui.LogError("Failed to "+action, nil)
	}
}

// Image returns the image on the clipboard, or (nil, false) when the
// clipboard holds no image or reading it failed.
func (c clipboard) Image() (image.Image, bool) {
	var data []byte
	var format C.int
	runOnClipboardThread(func() {
		var ptr unsafe.Pointer
		var length C.int
		status := withClipboardRetry(func() C.int {
			return C.belluiWinClipboardGetImage(&ptr, &length, &format)
		})
		if status != C.BELLUI_CB_OK {
			logClipboardFailure("read the clipboard image", status)
			return
		}
		data = C.GoBytes(ptr, length)
		C.free(ptr)
	})
	if len(data) == 0 {
		return nil, false
	}

	switch format {
	case 1:
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			bellui.LogError("Failed to decode clipboard image", err)
			return nil, false
		}
		return img, true
	case 2:
		return internalclipboard.DecodeDIB(data)
	}
	return nil, false
}

// SetImage replaces the clipboard content with img, written as both the
// registered "PNG" format and CF_DIB for broad consumer compatibility.
func (c clipboard) SetImage(img image.Image) {
	if img == nil {
		bellui.LogError("Cannot write a nil image to the clipboard", nil)
		return
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		bellui.LogError("Failed to encode clipboard image", err)
		return
	}
	pngData := buf.Bytes()
	dibData := internalclipboard.EncodeDIB(img)

	runOnClipboardThread(func() {
		// The C side copies both buffers synchronously, so passing the Go
		// slices' backing arrays is safe here.
		status := withClipboardRetry(func() C.int {
			return C.belluiWinClipboardSetImage(
				unsafe.Pointer(&pngData[0]), C.int(len(pngData)),
				unsafe.Pointer(&dibData[0]), C.int(len(dibData)),
			)
		})
		logClipboardFailure("write the clipboard image", status)
	})
}

// Files returns the list of files on the clipboard, or nil when the
// clipboard holds no files.
func (c clipboard) Files() []bellui.URI {
	var joined []byte
	var count int
	runOnClipboardThread(func() {
		var buf *C.char
		var num, length C.int
		status := withClipboardRetry(func() C.int {
			return C.belluiWinClipboardGetFiles(&buf, &num, &length)
		})
		if status != C.BELLUI_CB_OK {
			logClipboardFailure("read the clipboard file list", status)
			return
		}
		joined = C.GoBytes(unsafe.Pointer(buf), length)
		count = int(num)
		C.free(unsafe.Pointer(buf))
	})
	if count == 0 {
		return nil
	}

	files := make([]bellui.URI, 0, count)
	for _, path := range bytes.Split(joined, []byte{0}) {
		if len(path) == 0 {
			continue
		}
		files = append(files, storage.NewFileURI(string(path)))
	}
	if len(files) == 0 {
		return nil
	}
	return files
}

// SetFiles replaces the clipboard content with the given files as a CF_HDROP
// list. Only file-scheme URIs can be placed on the system clipboard; others
// are skipped with a logged error.
func (c clipboard) SetFiles(files []bellui.URI) {
	paths := clipboardFilePaths(files)
	if len(paths) == 0 {
		return
	}

	// cgo forbids passing a Go array of Go pointers, so the string array is
	// assembled in C memory.
	cPaths, freePaths := cPathArray(paths)
	defer freePaths()

	runOnClipboardThread(func() {
		status := withClipboardRetry(func() C.int {
			return C.belluiWinClipboardSetFiles(cPaths, C.int(len(paths)))
		})
		logClipboardFailure("write the clipboard file list", status)
	})
}

// SetFlavors replaces the clipboard content with every set flavor of content
// in one Empty+SetClipboardData transaction (#583).
func (c clipboard) SetFlavors(content bellui.ClipboardContent) {
	var cPlain, cHTML *C.char
	if content.Text != "" {
		cPlain = C.CString(content.Text)
		defer C.free(unsafe.Pointer(cPlain))
	}
	if content.HTML != "" {
		cHTML = C.CString(string(internalclipboard.EncodeCFHTML(content.HTML)))
		defer C.free(unsafe.Pointer(cHTML))
	}

	var pngData, dibData []byte
	if content.Image != nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, content.Image); err != nil {
			bellui.LogError("Failed to encode clipboard image", err)
		} else {
			pngData = buf.Bytes()
			dibData = internalclipboard.EncodeDIB(content.Image)
		}
	}
	var cPNG, cDIB unsafe.Pointer
	if len(pngData) > 0 {
		cPNG = unsafe.Pointer(&pngData[0])
	}
	if len(dibData) > 0 {
		cDIB = unsafe.Pointer(&dibData[0])
	}

	paths := clipboardFilePaths(content.Files)
	cPaths, freePaths := cPathArray(paths)
	defer freePaths()

	runOnClipboardThread(func() {
		// The C side copies every buffer synchronously, so passing the Go
		// slices' backing arrays is safe here.
		status := withClipboardRetry(func() C.int {
			return C.belluiWinClipboardSetFlavors(
				cPlain, cHTML,
				cPNG, C.int(len(pngData)),
				cDIB, C.int(len(dibData)),
				cPaths, C.int(len(paths)),
			)
		})
		logClipboardFailure("write the clipboard flavors", status)
	})
}

// HTML returns the HTML markup on the clipboard, or ("", false) when the
// clipboard holds no HTML flavor.
func (c clipboard) HTML() (string, bool) {
	var payload []byte
	runOnClipboardThread(func() {
		var buf *C.char
		status := withClipboardRetry(func() C.int {
			return C.belluiWinClipboardGetHTML(&buf)
		})
		if status != C.BELLUI_CB_OK {
			logClipboardFailure("read the clipboard HTML", status)
			return
		}
		payload = []byte(C.GoString(buf))
		C.free(unsafe.Pointer(buf))
	})
	if len(payload) == 0 {
		return "", false
	}
	return internalclipboard.DecodeCFHTML(payload)
}

// SetHTML replaces the clipboard content with html wrapped in the CF_HTML
// header block, writing plainFallback as the CF_UNICODETEXT flavor of the
// same transaction.
func (c clipboard) SetHTML(html, plainFallback string) {
	cPayload := C.CString(string(internalclipboard.EncodeCFHTML(html)))
	defer C.free(unsafe.Pointer(cPayload))
	cPlain := C.CString(plainFallback)
	defer C.free(unsafe.Pointer(cPlain))

	runOnClipboardThread(func() {
		status := withClipboardRetry(func() C.int {
			return C.belluiWinClipboardSetHTML(cPayload, cPlain)
		})
		logClipboardFailure("write the clipboard HTML", status)
	})
}
