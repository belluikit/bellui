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
			h, cleanup := benchHost(b, depth)
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
	h, cleanup := benchHost(b, 8)
	defer cleanup()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.BaseWidget.Move(geo.NewPos(float32(i%40), float32(i%40)))
		h.sync()
	}
}

// benchHost puts a host inside depth nested containers inside a scroll — the
// representative shape, since a clipping ancestor is what makes the clip
// resolution do real work — and returns it laid out on a window.
func benchHost(b *testing.B, depth int) (*NativeViewHost, func()) {
	b.Helper()
	test.NewApp()

	h := NewNativeViewHost(benchNativeView{})
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
