//go:build (ci || android || ios || mobile) && (!windows || !ci) && !wasm

package mobile

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	intdriver "github.com/belluikit/bellui/internal/driver"
	"github.com/belluikit/bellui/widget"
)

// scrollPage lays out a page taller than the canvas and returns it.
func scrollPage(t *testing.T) (*canvas, *container.Scroll) {
	t.Helper()
	filler := belluicanvas.NewRectangle(color.Transparent)
	filler.SetMinSize(geo.NewSize(100, 400))
	page := container.NewVScroll(filler)
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(page)
	c.Resize(geo.NewSize(120, 200))
	require.Len(t, c.scrollablesAt(geo.NewPos(10, 10)), 1, "the page scrolls with nothing over it")
	return c, page
}

// A pan over a modal pop-up's scrim does not scroll the page beneath it, as a
// tap there does not reach it (#1173); one over the pop-up's own scroll
// scrolls that.
func Test_canvas_ModalScrimStopsScroll(t *testing.T) {
	c, _ := scrollPage(t)

	inner := belluicanvas.NewRectangle(color.Transparent)
	inner.SetMinSize(geo.NewSize(40, 200))
	own := container.NewVScroll(inner)
	own.SetMinSize(geo.NewSize(40, 40))
	pop := widget.NewModalPopUp(own, c)
	pop.Show()
	defer pop.Hide()

	assert.Empty(t, c.scrollablesAt(geo.NewPos(5, 5)), "the scrim stops the scroll")

	at := intdriver.AbsolutePositionForObject(own, c.ObjectTrees()).AddXY(20, 20)
	chain := c.scrollablesAt(at)
	if assert.NotEmpty(t, chain, "the pop-up's own scroll at %v", at) {
		assert.Equal(t, own, chain[0].Object)
	}
}

// A toast passes scrolls through to the page, as it does taps.
func Test_canvas_ToastPassesScroll(t *testing.T) {
	c, page := scrollPage(t)

	toast := widget.ShowToast("saved", c)
	defer toast.Dismiss()
	require.NotNil(t, c.Overlays().Top(), "the toast is shown")

	chain := c.scrollablesAt(geo.NewPos(10, 10))
	if assert.Len(t, chain, 1) {
		assert.Equal(t, page, chain[0].Object)
	}
}
