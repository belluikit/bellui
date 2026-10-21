package glfw

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// Test_glPaintWalk_SkipsByPaintExtent pins #602 on desktop: a text positioned
// but never resized is painted when the dirty region crosses its glyphs, and
// skipped when the region misses them.
func Test_glPaintWalk_SkipsByPaintExtent(t *testing.T) {
	test.NewTempApp(t)

	text := canvas.NewText("File Transfer", color.White)
	min := text.MinSize()
	pos := geo.NewPos(20, 20)

	w := &glPaintWalk{partialRepaint: true, drPos: geo.NewPos(0, 20+min.Height/2), drSize: geo.NewSize(300, 4)}
	assert.False(t, w.skipObjectForDirtyRegion(text, pos), "the region crosses the glyphs")
	assert.True(t, w.shouldSkipForDirtyRegion(pos, text.Size()), "precondition: the 0x0 box alone misses it")

	w.drPos = geo.NewPos(0, 20+min.Height+10)
	assert.True(t, w.skipObjectForDirtyRegion(text, pos), "a region below the glyphs skips the text")

	w.partialRepaint = false
	assert.False(t, w.skipObjectForDirtyRegion(text, pos), "a full repaint skips nothing")
}

// Test_glPaintWalk_EmptyRegionSkipsEverything pins #605 on desktop: a frame
// with nothing dirty has an empty region at (0, 0), and an object reaching
// across the origin, a shadow's blur or content scrolled past it, is not
// painted unclipped over the retained canvas.
func Test_glPaintWalk_EmptyRegionSkipsEverything(t *testing.T) {
	test.NewTempApp(t)

	shadow := canvas.NewBoxShadow(color.NRGBA{A: 0x40}, 8)
	shadow.Resize(geo.NewSize(200, 300))

	w := &glPaintWalk{partialRepaint: true}
	assert.True(t, w.skipObjectForDirtyRegion(shadow, geo.NewPos(0, 0)), "a shadow's blur across the origin")
	assert.True(t, w.shouldSkipForDirtyRegion(geo.NewPos(-40, -40), geo.NewSize(120, 120)), "content scrolled across the origin")
}
