//go:build darwin && !wasm

package glfw

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework AppKit
#include <stdlib.h>
#include "clipboard_flavors_darwin.h"
*/
import "C"

import (
	"bytes"
	"image"
	"image/png"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/storage"
)

// Declare conformity with the optional clipboard flavor interfaces (#513).
var (
	_ bellui.ImageClipboard       = clipboard{}
	_ bellui.FileClipboard        = clipboard{}
	_ bellui.HTMLClipboard        = clipboard{}
	_ bellui.MultiFlavorClipboard = clipboard{}
)

// Image returns the image on the clipboard, or (nil, false) when the
// clipboard holds no image or reading it failed.
func (c clipboard) Image() (image.Image, bool) {
	var data []byte
	runOnClipboardThread(func() {
		var length C.int
		ptr := C.belluiClipboardGetImage(&length)
		if ptr == nil {
			return
		}
		data = C.GoBytes(ptr, length)
		C.free(ptr)
	})
	if len(data) == 0 {
		return nil, false
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		bellui.LogError("Failed to decode clipboard image", err)
		return nil, false
	}
	return img, true
}

// SetImage replaces the clipboard content with img.
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

	data := buf.Bytes()
	runOnClipboardThread(func() {
		// The ObjC side copies the buffer synchronously, so passing the Go
		// slice's backing array is safe here.
		if !C.belluiClipboardSetImage(unsafe.Pointer(&data[0]), C.int(len(data))) {
			bellui.LogError("Failed to write image to the clipboard", nil)
		}
	})
}

// Files returns the list of files on the clipboard, or nil when the
// clipboard holds no files.
func (c clipboard) Files() []bellui.URI {
	var paths []string
	runOnClipboardThread(func() {
		var count C.int
		arr := C.belluiClipboardGetFiles(&count)
		if arr == nil {
			return
		}
		for _, p := range unsafe.Slice(arr, int(count)) {
			paths = append(paths, C.GoString(p))
			C.free(unsafe.Pointer(p))
		}
		C.free(unsafe.Pointer(arr))
	})

	if len(paths) == 0 {
		return nil
	}
	files := make([]bellui.URI, 0, len(paths))
	for _, path := range paths {
		files = append(files, storage.NewFileURI(path))
	}
	return files
}

// SetFiles replaces the clipboard content with the given files.
// Only file-scheme URIs can be placed on the system clipboard; others are
// skipped with a logged error.
func (c clipboard) SetFiles(files []bellui.URI) {
	paths := clipboardFilePaths(files)

	// cgo forbids passing a Go array of Go pointers, so the string array is
	// assembled in C memory.
	cPaths, freePaths := cPathArray(paths)
	defer freePaths()

	runOnClipboardThread(func() {
		if !C.belluiClipboardSetFiles(cPaths, C.int(len(paths))) && len(paths) > 0 {
			bellui.LogError("Failed to write files to the clipboard", nil)
		}
	})
}

// SetFlavors replaces the clipboard content with every set flavor of content
// in one pasteboard transaction (#583): one NSPasteboardItem carries the
// text/HTML/image flavors, plus one item per file URL.
func (c clipboard) SetFlavors(content bellui.ClipboardContent) {
	var cText, cHTML *C.char
	if content.Text != "" {
		cText = C.CString(content.Text)
		defer C.free(unsafe.Pointer(cText))
	}
	if content.HTML != "" {
		cHTML = C.CString(content.HTML)
		defer C.free(unsafe.Pointer(cHTML))
	}

	var pngData []byte
	if content.Image != nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, content.Image); err != nil {
			bellui.LogError("Failed to encode clipboard image", err)
		} else {
			pngData = buf.Bytes()
		}
	}
	var cPNG unsafe.Pointer
	if len(pngData) > 0 {
		cPNG = unsafe.Pointer(&pngData[0])
	}

	paths := clipboardFilePaths(content.Files)
	cPaths, freePaths := cPathArray(paths)
	defer freePaths()

	runOnClipboardThread(func() {
		// The ObjC side copies every buffer synchronously, so passing the Go
		// slice's backing array is safe here.
		if !C.belluiClipboardSetFlavors(cText, cHTML, cPNG, C.int(len(pngData)), cPaths, C.int(len(paths))) {
			bellui.LogError("Failed to write flavors to the clipboard", nil)
		}
	})
}

// HTML returns the HTML markup on the clipboard, or ("", false) when the
// clipboard holds no HTML flavor.
func (c clipboard) HTML() (string, bool) {
	html := ""
	found := false
	runOnClipboardThread(func() {
		ptr := C.belluiClipboardGetHTML()
		if ptr == nil {
			return
		}
		html = C.GoString(ptr)
		C.free(unsafe.Pointer(ptr))
		found = true
	})
	return html, found
}

// SetHTML replaces the clipboard content with html, writing plainFallback as
// the plain-text flavor of the same transaction.
func (c clipboard) SetHTML(html, plainFallback string) {
	cHTML := C.CString(html)
	defer C.free(unsafe.Pointer(cHTML))
	cPlain := C.CString(plainFallback)
	defer C.free(unsafe.Pointer(cPlain))

	runOnClipboardThread(func() {
		C.belluiClipboardSetHTML(cHTML, cPlain)
	})
}
