package mobile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui"
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
