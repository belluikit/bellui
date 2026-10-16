//go:build linux && !wasm

package glfw

import (
	"bytes"
	"image"
	"image/png"
	"strings"

	"github.com/belluikit/bellui"
	internalclipboard "github.com/belluikit/bellui/internal/clipboard"
	"github.com/belluikit/bellui/storage"

	"github.com/belluikit/glfw"
)

// Clipboard flavors on Linux (pre-rebrand tracker #513 phase 2) ride on the forked GLFW's
// generic MIME clipboard API (glfwSetClipboardData & co.), which owns the
// X11 selection / Wayland data source for both text and data flavors. The
// GLFW platform layer dispatches to X11 or Wayland at runtime.

// Declare conformity with the optional clipboard flavor interfaces (pre-rebrand tracker #513).
var (
	_ bellui.ImageClipboard       = clipboard{}
	_ bellui.FileClipboard        = clipboard{}
	_ bellui.HTMLClipboard        = clipboard{}
	_ bellui.MultiFlavorClipboard = clipboard{}
)

const (
	mimePNG        = "image/png"
	mimeHTML       = "text/html"
	mimeText       = "text/plain;charset=utf-8"
	mimeURIList    = "text/uri-list"
	mimeGnomeFiles = "x-special/gnome-copied-files"
)

// Image returns the image on the clipboard, or (nil, false) when the
// clipboard holds no image or reading it failed.
func (c clipboard) Image() (image.Image, bool) {
	var data []byte
	runOnClipboardThread(func() {
		data = glfw.GetClipboardData(mimePNG)
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

	runOnClipboardThread(func() {
		glfw.SetClipboardData([]glfw.ClipboardFlavor{
			{MIMEType: mimePNG, Data: buf.Bytes()},
		})
	})
}

// Files returns the list of files on the clipboard, or nil when the
// clipboard holds no files.
func (c clipboard) Files() []bellui.URI {
	var paths []string
	runOnClipboardThread(func() {
		// Prefer the file-manager flavor for its operation line, but fall
		// through to the standard text/uri-list when it is absent or bogus
		// (some owners serve their data for any requested target).
		if data := glfw.GetClipboardData(mimeGnomeFiles); len(data) > 0 {
			if _, p := internalclipboard.DecodeGnomeCopiedFiles(data); len(p) > 0 {
				paths = p
				return
			}
		}
		if data := glfw.GetClipboardData(mimeURIList); len(data) > 0 {
			paths = internalclipboard.DecodeURIList(data)
		}
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
	flavors := fileFlavors(clipboardFilePaths(files))
	runOnClipboardThread(func() {
		glfw.SetClipboardData(flavors)
	})
}

// fileFlavors builds both the standard file-list flavor and the GNOME/KDE
// file-manager one so paste works in Files/Dolphin as a copy operation.
func fileFlavors(paths []string) []glfw.ClipboardFlavor {
	return []glfw.ClipboardFlavor{
		{MIMEType: mimeURIList, Data: internalclipboard.EncodeURIList(paths)},
		{MIMEType: mimeGnomeFiles, Data: internalclipboard.EncodeGnomeCopiedFiles("copy", paths)},
	}
}

// SetFlavors replaces the clipboard content with every set flavor of content
// in one transaction (pre-rebrand tracker #583). The forked GLFW's MIME clipboard API is already
// transaction-native, so the struct maps 1:1 onto one SetClipboardData call.
func (c clipboard) SetFlavors(content bellui.ClipboardContent) {
	var flavors []glfw.ClipboardFlavor
	if content.Text != "" {
		flavors = append(flavors, glfw.ClipboardFlavor{MIMEType: mimeText, Data: []byte(content.Text)})
	}
	if content.HTML != "" {
		flavors = append(flavors, glfw.ClipboardFlavor{MIMEType: mimeHTML, Data: []byte(content.HTML)})
	}
	if content.Image != nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, content.Image); err != nil {
			bellui.LogError("Failed to encode clipboard image", err)
		} else {
			flavors = append(flavors, glfw.ClipboardFlavor{MIMEType: mimePNG, Data: buf.Bytes()})
		}
	}
	if paths := clipboardFilePaths(content.Files); len(paths) > 0 {
		flavors = append(flavors, fileFlavors(paths)...)
	}
	runOnClipboardThread(func() {
		glfw.SetClipboardData(flavors)
	})
}

// HTML returns the HTML markup on the clipboard, or ("", false) when the
// clipboard holds no HTML flavor.
func (c clipboard) HTML() (string, bool) {
	var data []byte
	runOnClipboardThread(func() {
		data = glfw.GetClipboardData(mimeHTML)
	})
	if len(data) == 0 {
		return "", false
	}
	return decodeClipboardHTML(data), true
}

// SetHTML replaces the clipboard content with html, writing plainFallback as
// the plain-text flavor of the same transaction.
func (c clipboard) SetHTML(html, plainFallback string) {
	flavors := []glfw.ClipboardFlavor{
		{MIMEType: mimeHTML, Data: []byte(html)},
		{MIMEType: mimeText, Data: []byte(plainFallback)},
	}
	runOnClipboardThread(func() {
		glfw.SetClipboardData(flavors)
	})
}

// decodeClipboardHTML interprets a text/html payload as UTF-8, converting
// from UTF-16 when a writer used that encoding (some X11 apps do, with BOM).
func decodeClipboardHTML(data []byte) string {
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE { // UTF-16LE BOM
		return utf16ToString(data[2:], false)
	}
	if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF { // UTF-16BE BOM
		return utf16ToString(data[2:], true)
	}
	// Strip a UTF-8 BOM if present.
	return strings.TrimPrefix(string(data), "\uFEFF")
}

func utf16ToString(data []byte, bigEndian bool) string {
	var b strings.Builder
	for i := 0; i+1 < len(data); i += 2 {
		var r rune
		if bigEndian {
			r = rune(data[i])<<8 | rune(data[i+1])
		} else {
			r = rune(data[i+1])<<8 | rune(data[i])
		}
		// Combine surrogate pairs.
		if r >= 0xD800 && r < 0xDC00 && i+3 < len(data) {
			var lo rune
			if bigEndian {
				lo = rune(data[i+2])<<8 | rune(data[i+3])
			} else {
				lo = rune(data[i+3])<<8 | rune(data[i+2])
			}
			if lo >= 0xDC00 && lo < 0xE000 {
				r = 0x10000 + (r-0xD800)<<10 + (lo - 0xDC00)
				i += 2
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}
