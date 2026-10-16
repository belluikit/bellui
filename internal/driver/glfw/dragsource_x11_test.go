//go:build linux && !wayland && !wasm

package glfw

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/driver/desktop"
	"github.com/belluikit/bellui/storage"
)

// mimes collects the MIME/target names of the built flavors, in order.
func mimes(flavors []dragFlavor) []string {
	out := make([]string, len(flavors))
	for i, f := range flavors {
		out[i] = f.mime
	}
	return out
}

// TestBuildX11Flavors_Text offers plain text under the several target atoms an
// XDND editor may request, all carrying the same UTF-8 bytes (pre-rebrand tracker #838).
func TestBuildX11Flavors_Text(t *testing.T) {
	flavors := buildDragFlavors(desktop.DragData{Text: "hi"})
	assert.Equal(t, []string{"text/plain;charset=utf-8", "UTF8_STRING", "text/plain", "STRING"}, mimes(flavors))
	for _, f := range flavors {
		assert.Equal(t, []byte("hi"), f.data)
	}
}

// TestBuildX11Flavors_Files carries only file:// URIs, as text/uri-list plus the
// GNOME/KDE file-manager flavor; non-file schemes are skipped.
func TestBuildX11Flavors_Files(t *testing.T) {
	httpURI, _ := storage.ParseURI("https://example.com/x") // non-file, skipped
	flavors := buildDragFlavors(desktop.DragData{
		URIs: []bellui.URI{
			storage.NewFileURI("/tmp/a.txt"),
			httpURI,
		},
	})
	assert.Equal(t, []string{"text/uri-list", "x-special/gnome-copied-files"}, mimes(flavors))
	assert.Contains(t, string(flavors[0].data), "file:///tmp/a.txt")
}

// TestBuildX11Flavors_Image encodes an image as a single image/png flavor.
func TestBuildX11Flavors_Image(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	flavors := buildDragFlavors(desktop.DragData{Image: img})
	assert.Equal(t, []string{"image/png"}, mimes(flavors))
	assert.NotEmpty(t, flavors[0].data)
}

// TestBuildX11Flavors_Empty yields no flavors when nothing is set, so
// startDragX11 declines and the caller keeps its in-app behaviour.
func TestBuildX11Flavors_Empty(t *testing.T) {
	assert.Empty(t, buildDragFlavors(desktop.DragData{}))
}
