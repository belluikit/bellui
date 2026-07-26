//go:build wasm

package glfw

import (
	"bytes"
	"image"
	"image/png"
	"syscall/js"

	"github.com/belluikit/bellui"
)

// Declare conformity with the optional clipboard flavor interfaces (#513).
// FileClipboard is deliberately not implemented: browsers expose no portable
// file-list clipboard flavor, so the capability assertion fails on wasm.
var (
	_ bellui.ImageClipboard       = clipboard{}
	_ bellui.HTMLClipboard        = clipboard{}
	_ bellui.MultiFlavorClipboard = clipboard{}
)

func jsClipboard() js.Value {
	nav := js.Global().Get("navigator")
	if !nav.Truthy() {
		return js.Undefined()
	}
	return nav.Get("clipboard")
}

// readClipboardData resolves navigator.clipboard.read() and returns the bytes
// of the first ClipboardItem carrying the given MIME type, or nil when the
// clipboard holds no such flavor or the browser refuses (no permission, no
// transient user activation). The whole promise chain lives in JS callbacks
// so the read starts synchronously in the caller's (post-gesture) turn; the
// calling goroutine blocks on a buffered channel, which yields to the JS
// event loop, and the callbacks never block (#113).
func readClipboardData(mime string) []byte {
	cb := jsClipboard()
	if !cb.Truthy() {
		return nil
	}

	result := make(chan []byte, 1)

	fail := js.FuncOf(func(this js.Value, args []js.Value) any {
		result <- nil
		return nil
	})
	defer fail.Release()

	handleBuffer := js.FuncOf(func(this js.Value, args []js.Value) any {
		array := js.Global().Get("Uint8Array").New(args[0])
		data := make([]byte, array.Length())
		js.CopyBytesToGo(data, array)
		result <- data
		return nil
	})
	defer handleBuffer.Release()

	handleBlob := js.FuncOf(func(this js.Value, args []js.Value) any {
		args[0].Call("arrayBuffer").Call("then", handleBuffer).Call("catch", fail)
		return nil
	})
	defer handleBlob.Release()

	handleItems := js.FuncOf(func(this js.Value, args []js.Value) any {
		items := args[0]
		for i := 0; i < items.Length(); i++ {
			item := items.Index(i)
			types := item.Get("types")
			for j := 0; j < types.Length(); j++ {
				if types.Index(j).String() == mime {
					item.Call("getType", mime).Call("then", handleBlob).Call("catch", fail)
					return nil
				}
			}
		}
		result <- nil
		return nil
	})
	defer handleItems.Release()

	cb.Call("read").Call("then", handleItems).Call("catch", fail)
	return <-result
}

// writeClipboardItem writes one ClipboardItem carrying all given flavors.
// Best-effort fire-and-forget: rejection (missing permission or activation)
// is logged, matching the text path's writeText behavior.
func writeClipboardItem(flavors map[string][]byte) {
	cb := jsClipboard()
	if !cb.Truthy() {
		bellui.LogError("Clipboard API is unavailable in this browser", nil)
		return
	}
	itemConstructor := js.Global().Get("ClipboardItem")
	if !itemConstructor.Truthy() {
		bellui.LogError("ClipboardItem is unavailable in this browser", nil)
		return
	}

	entries := map[string]any{}
	for mime, data := range flavors {
		array := js.Global().Get("Uint8Array").New(len(data))
		js.CopyBytesToJS(array, data)
		entries[mime] = js.Global().Get("Blob").New([]any{array}, map[string]any{"type": mime})
	}
	item := itemConstructor.New(js.ValueOf(entries))

	var settled, failed js.Func
	release := func() {
		settled.Release()
		failed.Release()
	}
	settled = js.FuncOf(func(this js.Value, args []js.Value) any {
		release()
		return nil
	})
	failed = js.FuncOf(func(this js.Value, args []js.Value) any {
		bellui.LogError("Failed to write to the clipboard", nil)
		release()
		return nil
	})
	cb.Call("write", js.ValueOf([]any{item})).Call("then", settled).Call("catch", failed)
}

// Image returns the image on the clipboard, or (nil, false) when the
// clipboard holds no image or the browser refuses access.
func (c clipboard) Image() (image.Image, bool) {
	data := readClipboardData("image/png")
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

// SetImage replaces the clipboard content with img as a PNG ClipboardItem.
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
	writeClipboardItem(map[string][]byte{"image/png": buf.Bytes()})
}

// HTML returns the HTML markup on the clipboard, or ("", false) when the
// clipboard holds no HTML flavor or the browser refuses access.
func (c clipboard) HTML() (string, bool) {
	data := readClipboardData("text/html")
	if len(data) == 0 {
		return "", false
	}
	return string(data), true
}

// SetHTML replaces the clipboard content with one ClipboardItem carrying
// both the HTML flavor and plainFallback as text/plain.
func (c clipboard) SetHTML(html, plainFallback string) {
	writeClipboardItem(map[string][]byte{
		"text/html":  []byte(html),
		"text/plain": []byte(plainFallback),
	})
}

// SetFlavors replaces the clipboard content with one ClipboardItem carrying
// every set flavor of content (#583). Browsers expose no portable file-list
// clipboard flavor, so Files is skipped with a logged error.
func (c clipboard) SetFlavors(content bellui.ClipboardContent) {
	if len(content.Files) > 0 {
		bellui.LogError("File flavors are not supported on the web clipboard, skipping", nil)
	}

	flavors := map[string][]byte{}
	if content.Text != "" {
		flavors["text/plain"] = []byte(content.Text)
	}
	if content.HTML != "" {
		flavors["text/html"] = []byte(content.HTML)
	}
	if content.Image != nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, content.Image); err != nil {
			bellui.LogError("Failed to encode clipboard image", err)
		} else {
			flavors["image/png"] = buf.Bytes()
		}
	}
	if len(flavors) == 0 {
		// An empty ClipboardItem is rejected by browsers; clear via the
		// plain-text path instead.
		c.SetContent("")
		return
	}
	writeClipboardItem(flavors)
}
