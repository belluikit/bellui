//go:build !no_glfw && !ci && !mobile && !wasm

package glfw

import (
	"runtime"
	"testing"
	"weak"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/widget"
)

// TestReleaseDroppedPage pins the driver's bellui.Release with a real window (#1083): a
// page swapped out of a container and released is let go of by the frame that
// shows the next one.
func TestReleaseDroppedPage(t *testing.T) {
	w := createWindow("Release")
	defer w.Close()
	state := &struct{ taps int }{}
	page := widget.NewButton("page", func() { state.taps++ })
	slot := container.NewStack(page)
	w.SetContent(container.NewBorder(widget.NewLabel("nav"), nil, nil, nil, slot))
	runOnMain(w.window.Show)
	repaintWindow(w)

	runOnMain(func() {
		slot.Objects = []core.CanvasObject{widget.NewLabel("next")}
		slot.Refresh()
		d.Release(page) // the glfw tests' app is not this driver
		assert.True(t, w.canvas.CheckDirtyAndClear(), "the release asks for no frame")
		w.canvas.SetDirty()
	})
	repaintWindow(w)

	runOnMain(func() {
		_, rendered := cache.CachedRenderer(page)
		assert.False(t, rendered, "the dropped page keeps its renderer")
	})
	ws := weak.Make(state)
	page, state = nil, nil
	runtime.GC()
	runtime.GC()
	assert.Nil(t, ws.Value(), "the dropped page is still reachable")
}
