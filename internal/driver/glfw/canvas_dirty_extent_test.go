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
