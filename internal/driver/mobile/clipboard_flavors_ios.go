//go:build ios && !wasm

package mobile

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework UIKit

#include <stdbool.h>
#include <stdlib.h>

bool belluiClipboardSetImage(void *data, int len);
void *belluiClipboardGetImage(int *outLen);
bool belluiClipboardSetFlavors(const char *text, const char *html, void *png, int pngLen);
*/
import "C"

import (
	"bytes"
	"image"
	"image/png"
	"unsafe"

	"github.com/belluikit/bellui"
)

// mobileClipboard carries raster images on iOS via UIPasteboard (pre-rebrand tracker #580) and
// writes combined flavors in one transaction (pre-rebrand tracker #1142). FileClipboard stays
// unimplemented on mobile (matching wasm) — the assertion for it fails.
var (
	_ bellui.ImageClipboard       = mobileClipboard{}
	_ bellui.MultiFlavorClipboard = mobileClipboard{}
)

// Image returns the image on the pasteboard, or (nil, false) when the pasteboard
// holds no image or decoding failed. The image travels as PNG bytes across the
// cgo boundary.
func (c mobileClipboard) Image() (image.Image, bool) {
	var length C.int
	ptr := C.belluiClipboardGetImage(&length)
	if ptr == nil {
		return nil, false
	}
	data := C.GoBytes(ptr, length)
	C.free(ptr)
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

// SetFlavors replaces the pasteboard content with every set flavor of content
// in one transaction (pre-rebrand tracker #1142): a single UIPasteboard item mapping UTIs to the
// text/HTML/PNG representations. Files are skipped with a logged error —
// mobile has no file-list clipboard flavor (FileClipboard is unimplemented).
func (c mobileClipboard) SetFlavors(content bellui.ClipboardContent) {
	if len(content.Files) > 0 {
		bellui.LogError("File flavors are not supported on the mobile clipboard, skipping", nil)
	}

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

	// The ObjC side copies every buffer synchronously, so passing the Go
	// slice's backing array is safe here.
	if !C.belluiClipboardSetFlavors(cText, cHTML, cPNG, C.int(len(pngData))) {
		bellui.LogError("Failed to write flavors to the clipboard", nil)
	}
}

// SetImage replaces the pasteboard content with img (PNG-encoded).
func (c mobileClipboard) SetImage(img image.Image) {
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
	// The ObjC side copies the buffer synchronously, so passing the Go slice's
	// backing array is safe here.
	if !C.belluiClipboardSetImage(unsafe.Pointer(&data[0]), C.int(len(data))) {
		bellui.LogError("Failed to write image to the clipboard", nil)
	}
}
