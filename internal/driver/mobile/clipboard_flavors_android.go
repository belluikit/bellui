//go:build android

package mobile

/*
#include <stdlib.h>

void belluiClipboardSetImage(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx, char *path);
char *belluiClipboardGetImage(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx);
void belluiClipboardSetFlavors(uintptr_t java_vm, uintptr_t jni_env, uintptr_t ctx,
	char *text, char *html, char *imagePath);
*/
import "C"

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"unsafe"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/internal/driver/mobile/app"
)

// mobileClipboard carries raster images on Android via the system
// ClipboardManager (#580) and writes combined flavors in one transaction
// (#1142). Because Android cannot place a raw bitmap on the clipboard, the
// image travels as a content:// URI: the PNG is staged in the
// BelluiFileProvider share directory (the #711 provider) and the ClipData carries
// its URI. FileClipboard stays unimplemented on mobile (matching wasm) — the
// assertion for it fails.
var (
	_ bellui.ImageClipboard       = mobileClipboard{}
	_ bellui.MultiFlavorClipboard = mobileClipboard{}
)

// stageClipboardPNG PNG-encodes img into the BelluiFileProvider serve directory
// (cacheDir/belluishare, which os.TempDir resolves to on Android) so the
// provider can vend it, returning the staged path or "" on failure (logged).
func stageClipboardPNG(img image.Image) string {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		bellui.LogError("Failed to encode clipboard image", err)
		return ""
	}

	dir := filepath.Join(os.TempDir(), "belluishare")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		bellui.LogError("Failed to make clipboard cache dir", err)
		return ""
	}
	path := filepath.Join(dir, "bellui-clip-out.png")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		bellui.LogError("Failed to stage clipboard image", err)
		return ""
	}
	return path
}

// SetImage replaces the clipboard content with img (PNG-encoded, vended through
// BelluiFileProvider as an image/png content:// URI).
func (c mobileClipboard) SetImage(img image.Image) {
	if img == nil {
		bellui.LogError("Cannot write a nil image to the clipboard", nil)
		return
	}
	path := stageClipboardPNG(img)
	if path == "" {
		return
	}

	pathStr := C.CString(path)
	defer C.free(unsafe.Pointer(pathStr))
	app.RunOnJVM(func(vm, env, ctx uintptr) error {
		C.belluiClipboardSetImage(C.uintptr_t(vm), C.uintptr_t(env), C.uintptr_t(ctx), pathStr)
		return nil
	})
}

// SetFlavors replaces the clipboard content with every set flavor of content
// in one transaction (#1142): a single ClipData item natively carries text +
// HTML + an image content:// URI, with each present MIME listed in the
// ClipDescription. Files are skipped with a logged error — mobile has no
// file-list clipboard flavor (FileClipboard is unimplemented).
func (c mobileClipboard) SetFlavors(content bellui.ClipboardContent) {
	if len(content.Files) > 0 {
		bellui.LogError("File flavors are not supported on the mobile clipboard, skipping", nil)
	}

	var cText, cHTML, cPath *C.char
	if content.Text != "" {
		cText = C.CString(content.Text)
		defer C.free(unsafe.Pointer(cText))
	}
	if content.HTML != "" {
		cHTML = C.CString(content.HTML)
		defer C.free(unsafe.Pointer(cHTML))
	}
	if content.Image != nil {
		if path := stageClipboardPNG(content.Image); path != "" {
			cPath = C.CString(path)
			defer C.free(unsafe.Pointer(cPath))
		}
	}

	app.RunOnJVM(func(vm, env, ctx uintptr) error {
		C.belluiClipboardSetFlavors(C.uintptr_t(vm), C.uintptr_t(env), C.uintptr_t(ctx), cText, cHTML, cPath)
		return nil
	})
}

// Image returns the image on the clipboard, or (nil, false) when the clipboard
// holds no image or decoding failed. The Java side reads the clipboard's image
// URI, re-encodes it to a PNG temp file and returns the path; Go decodes it.
func (c mobileClipboard) Image() (image.Image, bool) {
	var path string
	app.RunOnJVM(func(vm, env, ctx uintptr) error {
		chars := C.belluiClipboardGetImage(C.uintptr_t(vm), C.uintptr_t(env), C.uintptr_t(ctx))
		if chars == nil {
			return nil
		}
		path = C.GoString(chars)
		C.free(unsafe.Pointer(chars))
		return nil
	})
	if path == "" {
		return nil, false
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		bellui.LogError("Failed to read clipboard image", err)
		return nil, false
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		bellui.LogError("Failed to decode clipboard image", err)
		return nil, false
	}
	return img, true
}
