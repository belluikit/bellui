package mobile

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui"
	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
)

// The screen-reader focus ring is painted over the content after the walk,
// and is no canvas object, so under partial repaint the driver marks where it
// is and where it was: a moved ring left its old pixels in the retained canvas
// texture, and a ring that stayed was painted over itself every frame (#551).
func TestMarkA11yRingDirty(t *testing.T) {
	content := container.NewWithoutLayout(belluicanvas.NewRectangle(color.White))
	content.Resize(geo.NewSize(400, 400))
	c := newCanvas(bellui.CurrentDevice()).(*canvas)
	c.SetContent(content)
	c.Resize(geo.NewSize(400, 400))
	c.PartialDirtyRegion()
	d := &driver{}
	covers := func(pos geo.Position, sz geo.Size, r geo.Rectangle) bool {
		h := float32(a11yFocusRingHalo)
		return pos.X <= r.Pos.X-h && pos.Y <= r.Pos.Y-h &&
			pos.X+sz.Width >= r.Pos.X+r.Size.Width+h && pos.Y+sz.Height >= r.Pos.Y+r.Size.Height+h
	}

	first := geo.Rectangle{Pos: geo.NewPos(50, 50), Size: geo.NewSize(80, 30)}
	d.markA11yRingDirty(c, first, true)
	pos, sz, set := c.PartialDirtyRegion()
	require.True(t, set)
	assert.True(t, covers(pos, sz, first), "the ring and its halo are redrawn")
	d.a11yRingPainted, d.a11yRingWasPainted = first, true

	moved := geo.Rectangle{Pos: geo.NewPos(200, 260), Size: geo.NewSize(80, 30)}
	d.markA11yRingDirty(c, moved, true)
	pos, sz, set = c.PartialDirtyRegion()
	require.True(t, set)
	assert.True(t, covers(pos, sz, first), "where the ring was is cleared")
	assert.True(t, covers(pos, sz, moved), "where it is now is redrawn")
	d.a11yRingPainted = moved

	d.markA11yRingDirty(c, geo.Rectangle{}, false)
	pos, sz, set = c.PartialDirtyRegion()
	require.True(t, set)
	assert.True(t, covers(pos, sz, moved), "a ring that went away is cleared")
}
