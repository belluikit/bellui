//go:build (!windows || !ci) && !wasm

package mobile

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal"
	"github.com/belluikit/bellui/internal/cache"
	"github.com/belluikit/bellui/widget"
)

// statefulWidget's renderer counts the times it is destroyed, as a renderer
// holding a decoder or a stream would notice.
type statefulWidget struct {
	widget.BaseWidget

	destroyed int
}

func newStatefulWidget() *statefulWidget {
	w := &statefulWidget{}
	w.ExtendBaseWidget(w)
	return w
}

func (w *statefulWidget) CreateRenderer() core.WidgetRenderer {
	return &statefulRenderer{w: w, rect: belluicanvas.NewRectangle(nil)}
}

type statefulRenderer struct {
	w    *statefulWidget
	rect *belluicanvas.Rectangle
}

func (r *statefulRenderer) Destroy()                     { r.w.destroyed++ }
func (r *statefulRenderer) Layout(size geo.Size)         { r.rect.Resize(size) }
func (r *statefulRenderer) MinSize() geo.Size            { return geo.NewSize(10, 10) }
func (r *statefulRenderer) Objects() []core.CanvasObject { return []core.CanvasObject{r.rect} }
func (r *statefulRenderer) Refresh()                     {}

// TestPaintedWidgetKeepsItsRendererPastValidDuration is #1146's first
// done-when on mobile: a widget the paint walk reaches every frame keeps its
// renderer however long it is painted, although each Clean destroys what was
// last looked up more than ValidDuration before.
func TestPaintedWidgetKeepsItsRendererPastValidDuration(t *testing.T) {
	now := time.Date(2026, time.November, 11, 0, 0, 0, 0, time.UTC)
	cache.SetClockForTests(t, func() time.Time { return now })

	w := newStatefulWidget()
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(container.NewVBox(widget.NewLabel("above"), w))
	c.Resize(geo.NewSize(100, 100))
	c.SetPainter(&clipRecordingPainter{})
	walk := &mobileTreeWalker{c: c, clips: &internal.ClipStack{}, size: geo.NewSize(100, 100)}
	// What handlePaint does with a dirty canvas: walk the trees to paint, then
	// clean the caches with the canvas refreshed.
	frame := func() {
		c.WalkTrees(walk.draw, walk.afterDraw)
		cache.Clean(true)
	}
	frame()
	first, ok := cache.CachedRenderer(w)
	require.True(t, ok, "precondition: the walk made the widget's renderer")

	// Clean runs at most every 10 s; step further than that and than
	// ValidDuration, so that every frame's Clean expires what the last frame's
	// walk did not reach.
	for i := 0; i < 5; i++ {
		now = now.Add(cache.ValidDuration + 11*time.Second)
		frame()
	}

	r, ok := cache.CachedRenderer(w)
	assert.True(t, ok, "the painted widget lost its renderer")
	assert.Same(t, first, r, "the painted widget got a new renderer")
	assert.Zero(t, w.destroyed, "the painted widget's renderer was destroyed")

	// The control: without the walk, the next Clean destroys it.
	now = now.Add(cache.ValidDuration + 11*time.Second)
	cache.Clean(true)
	now = now.Add(cache.ValidDuration + 11*time.Second)
	cache.Clean(true)
	assert.Equal(t, 1, w.destroyed, "a renderer no walk reached outlived ValidDuration")
}
