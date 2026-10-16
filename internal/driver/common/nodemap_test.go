package common

import (
	"image/color"
	"runtime"
	"testing"
	"weak"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// Content that leaves the canvas must not stay reachable from it. nodeMap
// only ever gained entries, so every object a window had drawn — every page
// a gallery-style app had shown, with all its data — stayed in memory for the
// window's lifetime.
func TestCanvas_nodeMapForgetsRemovedContent(t *testing.T) {
	test.NewTempApp(t)
	c := &Canvas{}
	c.Initialize(sizedCanvas{test.NewCanvas()}, func() {})

	old := canvas.NewRectangle(color.Gray16{Y: 1})
	c.SetContentTreeAndFocusMgr(container.NewVBox(old))
	c.WalkTrees(nil, nil)
	_, ok := c.nodeMap.Load(old)
	assert.True(t, ok, "precondition: the walked object is in nodeMap")

	c.SetContentTreeAndFocusMgr(container.NewVBox(canvas.NewRectangle(color.Gray16{Y: 2})))
	c.WalkTrees(nil, nil)

	stale := 0
	c.nodeMap.Range(func(obj core.CanvasObject, _ *RenderCacheNode) bool {
		if obj == core.CanvasObject(old) {
			stale++
		}
		return true
	})
	assert.Zero(t, stale, "removed object is still in nodeMap")

	w := weak.Make(old)
	old = nil
	runtime.GC()
	runtime.GC()
	assert.Nil(t, w.Value(), "removed object is still reachable after GC")
}

// sizedCanvas gives Canvas the SizeableCanvas it needs for the spatial index.
type sizedCanvas struct{ core.Canvas }

func (sizedCanvas) Resize(geo.Size)   {}
func (sizedCanvas) MinSize() geo.Size { return geo.NewSize(0, 0) }
func (sizedCanvas) Size() geo.Size    { return geo.NewSize(100, 100) }
