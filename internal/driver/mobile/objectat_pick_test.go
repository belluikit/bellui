//go:build !wasm

package mobile

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	pubdriver "github.com/belluikit/bellui/driver"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/widget"
)

// The public pick wrappers probe through the canvas's own dispatch hit-walk
// (#245), so roots the plain overlays+content walk misses — the window head
// of a child window, an open menu — are reachable, with bounds resolved in
// the Driver.AbsolutePositionForObject space (#235).
func Test_mobileCanvas_PublicPickCoversDispatchRoots(t *testing.T) {
	d := &driver{}

	w1 := d.CreateWindow("main").(*window)
	w1.SetContent(widget.NewLabel("main"))
	w1.Show()

	w2 := d.CreateWindow("child").(*window)
	w2.SetContent(widget.NewLabel("child"))
	w2.Show()
	w2.Resize(geo.NewSize(200, 300))

	mc := w2.Canvas().(*canvas)
	head := mc.windowHead
	require.NotNil(t, head, "a child window over a visible one gets a window head")

	// Compose the probe the way the doc comment tells tooling to: from
	// Driver.AbsolutePositionForObject. Resolving the head at all also
	// depends on ObjectTrees including the window-head tree (#245).
	absHead := d.AbsolutePositionForObject(head)
	obj, bounds, ok := pubdriver.ObjectAtPositionMatching(mc, absHead.AddXY(2, 2),
		func(o core.CanvasObject) bool { return o == head })
	require.True(t, ok, "the public pick reaches the window head")
	assert.Same(t, head, obj)
	assert.Equal(t, absHead, bounds.Pos)
	assert.Equal(t, head.Size(), bounds.Size)

	// With a menu open the dispatch walk serves overlays + menu only; the
	// public pick follows it there too.
	menu := belluicanvas.NewRectangle(color.Black)
	mc.setMenu(menu)
	menu.Resize(geo.NewSize(120, 300))

	absMenu := d.AbsolutePositionForObject(menu)
	obj, bounds, ok = pubdriver.ObjectAtPositionMatching(mc, absMenu.AddXY(2, 2),
		func(o core.CanvasObject) bool { return o == menu })
	require.True(t, ok, "the public pick reaches an open menu")
	assert.Same(t, menu, obj)
	assert.Equal(t, absMenu, bounds.Pos)
}
