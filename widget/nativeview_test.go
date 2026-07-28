package widget_test

import (
	"errors"
	"image/color"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
	"github.com/belluikit/bellui/widget"
	"github.com/stretchr/testify/assert"
)

// stubNativeView records what a NativeViewHost asks of its backend, standing
// in for a platform view in tests (#45).
type stubNativeView struct {
	attachErr error

	attached  int
	destroyed int
	visible   bool
	visSets   []bool
	rect      geo.Rectangle
	clip      geo.Rectangle
	scale     float32
	geoSets   int
}

func (s *stubNativeView) Attach(bellui.Window) error {
	s.attached++
	return s.attachErr
}

func (s *stubNativeView) SetGeometry(rect, clip geo.Rectangle, scale float32) {
	s.rect, s.clip, s.scale = rect, clip, scale
	s.geoSets++
}

func (s *stubNativeView) SetVisible(v bool) {
	s.visible = v
	s.visSets = append(s.visSets, v)
}

func (s *stubNativeView) Destroy() { s.destroyed++ }

// hostInWindow puts a sized host on a real test window, which is what gives
// it a canvas, an absolute position and a driver to resolve them.
func hostInWindow(t *testing.T, view widget.NativeView, size geo.Size) (*widget.NativeViewHost, bellui.Window) {
	t.Helper()
	test.NewTempApp(t)

	h := widget.NewNativeViewHost(view)
	w := test.NewTempWindow(t, container.NewWithoutLayout(h))
	w.Resize(size)
	h.Resize(geo.NewSize(100, 50))
	h.Move(geo.NewPos(10, 20))
	return h, w
}

func TestNativeViewHost_AttachesAndPlaces(t *testing.T) {
	v := &stubNativeView{}
	h, w := hostInWindow(t, v, geo.NewSize(400, 300))

	assert.Equal(t, 1, v.attached, "attached exactly once")
	assert.True(t, v.visible, "a laid-out host shows its native view")
	assert.Equal(t, geo.NewSize(100, 50), v.rect.Size)
	assert.Equal(t, w.Canvas().Scale(), v.scale)

	// Without a clipping ancestor the clip is the whole canvas, so the
	// visible region is just the host rectangle.
	assert.Equal(t, v.rect, v.clip)

	// The reported position is canvas-absolute, so it carries the window's
	// content inset; what matters is that it tracks the widget exactly.
	before := v.rect.Pos
	h.Move(geo.NewPos(30, 40))
	assert.Equal(t, before.AddXY(20, 20), v.rect.Pos, "a move re-places the view")
}

func TestNativeViewHost_FailedAttachLeavesHostInert(t *testing.T) {
	v := &stubNativeView{attachErr: errors.New("no engine here")}
	h, _ := hostInWindow(t, v, geo.NewSize(400, 300))

	assert.Equal(t, 1, v.attached)
	assert.Equal(t, 0, v.geoSets, "no geometry is pushed to a view that failed to attach")
	assert.False(t, v.visible)

	h.Move(geo.NewPos(30, 40)) // must not retry or panic
	assert.Equal(t, 1, v.attached, "attach is attempted once")
}

func TestNativeViewHost_HideShow(t *testing.T) {
	v := &stubNativeView{}
	h, _ := hostInWindow(t, v, geo.NewSize(400, 300))
	assert.True(t, v.visible)

	h.Hide()
	assert.False(t, v.visible, "hiding the host hides the native view")

	h.Show()
	assert.True(t, v.visible, "showing it again brings the native view back")
}

func TestNativeViewHost_ClipsToScrollAndHidesWhenScrolledOut(t *testing.T) {
	test.NewTempApp(t)

	v := &stubNativeView{}
	h := widget.NewNativeViewHost(v)
	h.SetMinSize(geo.NewSize(200, 400))

	// A tall sibling below the host gives the scroll enough travel to move
	// the host fully past the top of the viewport.
	tail := canvas.NewRectangle(color.Transparent)
	tail.SetMinSize(geo.NewSize(200, 2000))

	scroll := container.NewScroll(container.NewVBox(h, tail))
	w := test.NewTempWindow(t, scroll)
	w.Resize(geo.NewSize(300, 100))

	assert.True(t, v.visible)
	// The scroll clips the 400-tall host to the 100-tall viewport.
	assert.Less(t, v.clip.Size.Height, v.rect.Size.Height,
		"the reported clip is smaller than the host, so the backend can mask it")
	assert.LessOrEqual(t, v.clip.Size.Height, float32(100))

	// Scroll past the host: it leaves the viewport entirely.
	scroll.Offset = geo.NewPos(0, 1000)
	scroll.Refresh()
	h.Refresh()
	assert.False(t, v.visible, "a host scrolled out of its viewport hides the native view")

	// ...and comes back.
	scroll.Offset = geo.NewPos(0, 0)
	scroll.Refresh()
	h.Refresh()
	assert.True(t, v.visible, "scrolling back into view shows it again")
}

func TestNativeViewHost_DestroyTearsDownOnce(t *testing.T) {
	test.NewTempApp(t)

	v := &stubNativeView{}
	h := widget.NewNativeViewHost(v)
	box := container.NewVBox(h)
	w := test.NewTempWindow(t, box)
	w.Resize(geo.NewSize(400, 300))
	assert.Equal(t, 1, v.attached)

	// Replacing the content destroys the renderer, which tears the view down.
	w.SetContent(widget.NewLabel("gone"))
	test.TempWidgetRenderer(t, h).Destroy()

	assert.GreaterOrEqual(t, v.destroyed, 1, "the native view is destroyed")

	h.Refresh() // must not panic or resurrect anything after teardown
}

func TestNativeViewHost_NoBackend(t *testing.T) {
	test.NewTempApp(t)

	h := widget.NewNativeViewHost(nil)
	w := test.NewTempWindow(t, h)
	w.Resize(geo.NewSize(200, 100))

	h.Move(geo.NewPos(5, 5))
	h.Hide()
	h.Show()
	// The point is simply that a host with no backend is harmless.
	syncs, applies := h.SyncStats()
	assert.Zero(t, syncs)
	assert.Zero(t, applies)
}

// TestNativeViewHost_SteadyStateDoesNotTouchBackend is the cost claim behind
// the per-frame ticker: re-evaluating geometry every frame is cheap because a
// still host pushes nothing to the platform.
func TestNativeViewHost_SteadyStateDoesNotTouchBackend(t *testing.T) {
	v := &stubNativeView{}
	h, _ := hostInWindow(t, v, geo.NewSize(400, 300))

	before := v.geoSets
	for i := 0; i < 100; i++ {
		h.Refresh()
	}
	assert.Equal(t, before, v.geoSets, "an unmoved host pushes no geometry")

	syncs, applies := h.SyncStats()
	assert.Greater(t, syncs, applies, "most evaluations are filtered out before the backend")
}

// BenchmarkNativeViewHostSync measures one per-frame geometry evaluation: the
// visible-tree walk, absolute position and clip-bounds resolution a
// NativeViewHost does on every frame while shown (#45).
func BenchmarkNativeViewHostSync(b *testing.B) {
	test.NewApp()

	v := &stubNativeView{}
	h := widget.NewNativeViewHost(v)
	h.SetMinSize(geo.NewSize(200, 200))
	scroll := container.NewScroll(container.NewVBox(h))
	w := test.NewWindow(scroll)
	defer w.Close()
	w.Resize(geo.NewSize(400, 300))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Refresh()
	}
}
