package widget

import (
	"fmt"
	"testing"

	"github.com/belluikit/bellui"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	intWidget "github.com/belluikit/bellui/internal/widget"
	"github.com/belluikit/bellui/test"
)

// benchNativeView is an inert backend: the benchmarks below measure the
// host's own per-frame geometry resolution, not a platform view.
type benchNativeView struct{}

func (benchNativeView) Attach(bellui.Window) error                { return nil }
func (benchNativeView) SetGeometry(_, _ geo.Rectangle, _ float32) {}
func (benchNativeView) SetVisible(bool)                           {}
func (benchNativeView) Destroy()                                  {}

// benchFocusNativeView adds the optional focus capability, so a benchmark can
// price what the focus bridge adds to the same per-frame sync (#59). Its
// answers are constant: what is being measured is the host's side of the
// poll, since the platform side is one responder-chain query.
type benchFocusNativeView struct{ benchNativeView }

func (benchFocusNativeView) TakeFocus() bool   { return false }
func (benchFocusNativeView) ResignFocus() bool { return true }
func (benchFocusNativeView) HasFocus() bool    { return false }

// BenchmarkNativeViewHostSyncDepth measures one per-frame sync — the visible
// tree walk, absolute-position and clip-bounds resolution a NativeViewHost
// runs every frame while shown (#45).
//
// The walk is O(tree), so the cost is reported against nesting depth: this is
// the number that decides whether a per-frame ticker is an acceptable way to
// keep an OS view glued to a scrolling widget.
func BenchmarkNativeViewHostSyncDepth(b *testing.B) {
	for _, depth := range []int{1, 8, 32, 128} {
		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			h, cleanup := benchHost(b, depth, benchNativeView{})
			defer cleanup()

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				h.sync()
			}
		})
	}
}

// BenchmarkNativeViewHostSyncMoving is the worst case: the host moves every
// frame, so every sync reaches the backend instead of being filtered out by
// the unchanged-geometry check.
func BenchmarkNativeViewHostSyncMoving(b *testing.B) {
	h, cleanup := benchHost(b, 8, benchNativeView{})
	defer cleanup()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.BaseWidget.Move(geo.NewPos(float32(i%40), float32(i%40)))
		h.sync()
	}
}

// BenchmarkNativeViewHostSyncFocus is the same sync with a backend that can
// hold the OS keyboard focus, so the focus bridge's per-frame poll (#59) is
// priced against the depth=8 case above rather than assumed to be free.
func BenchmarkNativeViewHostSyncFocus(b *testing.B) {
	h, cleanup := benchHost(b, 8, benchFocusNativeView{})
	defer cleanup()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.sync()
	}
}

// benchHost puts a host inside depth nested containers inside a scroll — the
// representative shape, since a clipping ancestor is what makes the clip
// resolution do real work — and returns it laid out on a window.
func benchHost(b *testing.B, depth int, view NativeView) (*NativeViewHost, func()) {
	b.Helper()
	test.NewApp()

	h := NewNativeViewHost(view)
	h.SetMinSize(geo.NewSize(200, 200))
	h.Resize(geo.NewSize(200, 200))

	var content core.CanvasObject = h
	for i := 0; i < depth; i++ {
		c := &bellui.Container{Objects: []core.CanvasObject{content}}
		c.Resize(geo.NewSize(200, 200))
		content = c
	}

	scroll := intWidget.NewVScroll(content)
	w := test.NewWindow(scroll)
	w.Resize(geo.NewSize(300, 100))

	h.sync() // attach before timing
	return h, func() { w.Close() }
}

// TestNativeViewHost_FirstAttachSurvivesPrePaint is #167: the host's widget
// events fire during the initial SetContent layout, before the driver has
// painted and cached the tree, so CanvasForObject answers nil. Treating that
// like a detach stopped the per-frame ticker, and on a screen where nothing
// else moves no later event re-entered sync — the platform view never
// attached and the window stayed blank.
func TestNativeViewHost_FirstAttachSurvivesPrePaint(t *testing.T) {
	test.NewTempApp(t)

	h := NewNativeViewHost(benchNativeView{})
	// Not in any window: exactly what the pre-paint window looks like from
	// here, and what a driver answers before the tree is cached.
	h.sync()

	if h.anim == nil {
		t.Fatal("the ticker must be armed while the host has never attached, " +
			"or nothing re-enters sync on a static screen (#167)")
	}
	if h.shown {
		t.Error("an unresolved host must not be reported as shown")
	}
}

// Once the host HAS attached, the same unresolved answer means a genuine
// detach — hidden, ancestor hidden, removed — and the ticker stops, to be
// re-armed by the widget event that reverses it. That is the behaviour #167's
// fix must not trade away.
func TestNativeViewHost_DetachAfterAttachStopsTicker(t *testing.T) {
	test.NewTempApp(t)

	h := NewNativeViewHost(benchNativeView{})
	h.startSync()
	h.attached = true // as a successful first attach would leave it

	h.unresolved()

	if h.anim != nil {
		t.Error("a real detach must stop the ticker")
	}
	if h.shown {
		t.Error("a detached host must not be reported as shown")
	}
}

// destroy() is what bounds the pre-attach ticker: the renderer calls it when
// the host leaves the tree or its window closes.
func TestNativeViewHost_DestroyStopsPreAttachTicker(t *testing.T) {
	test.NewTempApp(t)

	h := NewNativeViewHost(benchNativeView{})
	h.sync() // arms the ticker, never attached
	if h.anim == nil {
		t.Fatal("precondition: the ticker should be armed")
	}

	h.destroy()

	if h.anim != nil {
		t.Error("destroy must stop the ticker, so an orphaned host cannot tick forever")
	}
}
