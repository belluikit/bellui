//go:build linux || freebsd || openbsd || netbsd

package glfw

import (
	"bytes"
	"image"
	"image/png"

	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/internal/build"
	internalclipboard "github.com/belluikit/bellui/internal/clipboard"
)

// startNativeDrag begins a native drag-and-drop out of the app on Linux and the
// BSDs (#838, #854, #515 Part B). GLFW exposes no upstream drag-source API, so
// each backend is hand-rolled. The default linux binary carries both backends
// and picks one at runtime via build.IsWayland (#542); the BSDs are X11-only, so
// build.IsWayland is always false there and the X11 source is used:
//
//   - X11: a hand-rolled XDND source (dragsource_x11.go / .c) drives the protocol
//     on a private Xlib connection.
//   - Wayland: dragsource_wayland.go drives wl_data_device.start_drag via the
//     belluikit/glfw fork's StartWaylandDrag (v1.7.0+, which owns the seat, data
//     device and pointer serial that start_drag requires). No completion signal
//     is exposed, so DragData.OnCompleted is not delivered on Wayland.
//
// Both backends share the flavor codecs below (buildDragFlavors), reusing the
// #513 clipboard MIME encoders.
func startNativeDrag(data desktop.DragData, cursorImg image.Image) bool {
	if build.IsWayland {
		return startDragWayland(data)
	}
	return startDragX11(data, cursorImg)
}

// dragFlavor is one MIME representation of the drag payload, encoded with the
// shared #513 clipboard codecs and served to the drop target (the X11 XDND
// selection or the Wayland wl_data_source) on demand.
type dragFlavor struct {
	mime string
	data []byte
}

// buildDragFlavors encodes the drag payload into the MIME flavors a drop target
// may request, reusing the shared #513 clipboard codecs. Text is offered under
// the several target atoms editors ask for; files as text/uri-list plus the
// GNOME/KDE file-manager flavor; an explicit image as PNG. The drag ghost is NOT
// included — it is a cursor visual, not a data flavor (#842).
func buildDragFlavors(data desktop.DragData) []dragFlavor {
	var out []dragFlavor
	if data.Text != "" {
		b := []byte(data.Text)
		for _, m := range []string{"text/plain;charset=utf-8", "UTF8_STRING", "text/plain", "STRING"} {
			out = append(out, dragFlavor{mime: m, data: b})
		}
	}
	if paths := filePaths(data.URIs); len(paths) > 0 {
		out = append(
			out,
			dragFlavor{mime: "text/uri-list", data: internalclipboard.EncodeURIList(paths)},
			dragFlavor{mime: "x-special/gnome-copied-files", data: internalclipboard.EncodeGnomeCopiedFiles("copy", paths)},
		)
	}
	if data.Image != nil {
		if b := encodePNG(data.Image); len(b) > 0 {
			out = append(out, dragFlavor{mime: "image/png", data: b})
		}
	}
	return out
}

// encodePNG renders an image to PNG bytes for the image/png flavor, or nil.
func encodePNG(img image.Image) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}
