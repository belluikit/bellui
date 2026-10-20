//go:build !wasm

package mobile

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// sizedWidget reports a min size its renderer holds, changed without
// refreshing any part: only the widget's own Refresh says it changed.
type sizedWidget struct {
	widget.BaseWidget
	min      geo.Size
	measured int // how many times the renderer was asked for its min size
}

func newSizedWidget(min geo.Size) *sizedWidget {
	w := &sizedWidget{min: min}
	w.ExtendBaseWidget(w)
	return w
}

func (w *sizedWidget) CreateRenderer() core.WidgetRenderer { return &sizedRenderer{w: w} }

type sizedRenderer struct{ w *sizedWidget }

func (r *sizedRenderer) Destroy()                     {}
func (r *sizedRenderer) Layout(geo.Size)              {}
func (r *sizedRenderer) Objects() []core.CanvasObject { return nil }
func (r *sizedRenderer) Refresh()                     {} // refreshes no part
func (r *sizedRenderer) MinSize() geo.Size {
	r.w.measured++
	return r.w.min
}

// TestEnsureMinSizeSkipsUnchangedWidgets pins #464 option 4: a widget nothing
// refreshed is not asked for its min size again.
func TestEnsureMinSizeSkipsUnchangedWidgets(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	w := newSizedWidget(geo.NewSize(100, 40))
	c.SetContent(container.NewStack(container.NewHBox(w)))
	c.Resize(geo.NewSize(400, 300))
	c.EnsureMinSize()
	c.EnsureMinSize() // settle: the first walks lay out and resize

	before := w.measured
	c.EnsureMinSize()
	c.EnsureMinSize()
	assert.Equal(t, before, w.measured, "an unchanged widget was asked for its min size again")
}

// TestEnsureMinSizeAfterWidgetRefresh checks the other half: after the
// widget's Refresh, its new min size is picked up and laid out, although its
// renderer refreshed no part the canvas would hear of.
func TestEnsureMinSizeAfterWidgetRefresh(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	w := newSizedWidget(geo.NewSize(100, 40))
	c.SetContent(container.NewStack(container.NewHBox(w)))
	c.Resize(geo.NewSize(400, 300))
	c.EnsureMinSize()
	assert.Equal(t, float32(100), w.Size().Width)

	w.min = geo.NewSize(150, 40)
	w.Refresh()
	c.EnsureMinSize()
	assert.Equal(t, float32(150), w.Size().Width, "the refreshed widget's new min size was not laid out")
}

// TestEnsureMinSizeAfterWidgetSetMinSize checks that a widget's SetMinSize,
// which refreshes nothing, is still laid out: SetMinSize makes the next pass
// ask everything again (cache.MinSizeSet).
func TestEnsureMinSizeAfterWidgetSetMinSize(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	scroll := container.NewVScroll(widget.NewLabel("side"))
	scroll.SetMinSize(geo.NewSize(100, 0))
	c.SetContent(container.NewStack(container.NewHBox(scroll)))
	c.Resize(geo.NewSize(400, 300))
	c.EnsureMinSize()
	c.EnsureMinSize()
	assert.Equal(t, float32(100), scroll.Size().Width)

	scroll.SetMinSize(geo.NewSize(240, 0))
	c.EnsureMinSize()
	assert.Equal(t, float32(240), scroll.Size().Width, "a widget's SetMinSize without a refresh was not laid out")
}

// TestEnsureMinSizeAfterHide checks that hiding a primitive, which refreshes
// nothing, still re-lays out its container.
func TestEnsureMinSizeAfterHide(t *testing.T) {
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	a := belluicanvas.NewRectangle(color.Black)
	a.SetMinSize(geo.NewSize(50, 50))
	b := belluicanvas.NewRectangle(color.White)
	b.SetMinSize(geo.NewSize(50, 50))
	c.SetContent(container.NewStack(container.NewHBox(a, b)))
	c.Resize(geo.NewSize(400, 300))
	c.EnsureMinSize()
	c.EnsureMinSize()
	assert.NotZero(t, b.Position().X)

	a.Hide()
	c.EnsureMinSize()
	assert.Zero(t, b.Position().X, "b should move into the hidden a's place")
}
