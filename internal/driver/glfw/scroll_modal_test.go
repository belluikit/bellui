//go:build !no_glfw && !mobile

package glfw

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/widget"
)

// scrollWindow is a window with only what scrollablesAt needs: a canvas
// holding a page taller than it.
func scrollWindow(t *testing.T) (*window, *container.Scroll) {
	t.Helper()
	filler := canvas.NewRectangle(color.Transparent)
	filler.SetMinSize(geo.NewSize(100, 400))
	page := container.NewVScroll(filler)
	c := newCanvas()
	c.SetContent(page)
	c.Resize(geo.NewSize(120, 200))
	w := &window{canvas: c}
	require.Len(t, w.scrollablesAt(geo.NewPos(10, 10)), 1, "the page scrolls with nothing over it")
	return w, page
}

// A wheel over a modal pop-up's scrim does not scroll the page beneath it, as
// a click there does not reach it (#1173); one over the pop-up's own scroll
// scrolls that.
func TestWindow_ModalScrimStopsScroll(t *testing.T) {
	w, _ := scrollWindow(t)

	inner := canvas.NewRectangle(color.Transparent)
	inner.SetMinSize(geo.NewSize(40, 200))
	own := container.NewVScroll(inner)
	own.SetMinSize(geo.NewSize(40, 40))
	pop := widget.NewModalPopUp(own, w.canvas)
	pop.Show()
	defer pop.Hide()

	assert.Empty(t, w.scrollablesAt(geo.NewPos(5, 5)), "the scrim stops the scroll")

	at := driver.AbsolutePositionForObject(own, w.canvas.ObjectTrees()).AddXY(20, 20)
	chain := w.scrollablesAt(at)
	if assert.NotEmpty(t, chain, "the pop-up's own scroll at %v", at) {
		assert.Equal(t, own, chain[0].Object)
	}
}

// A toast passes scrolls through to the page, as it does clicks.
func TestWindow_ToastPassesScroll(t *testing.T) {
	w, page := scrollWindow(t)

	toast := widget.ShowToast("saved", w.canvas)
	defer toast.Dismiss()
	require.NotNil(t, w.canvas.Overlays().Top(), "the toast is shown")

	chain := w.scrollablesAt(geo.NewPos(10, 10))
	if assert.Len(t, chain, 1) {
		assert.Equal(t, page, chain[0].Object)
	}
}
