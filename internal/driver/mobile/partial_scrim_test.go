//go:build (!windows || !ci) && !wasm

package mobile

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	belluicanvas "github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/build"
	"github.com/belluikit/bellui/internal/gpu"
	"github.com/belluikit/bellui/widget"
)

// freePainter takes the texture frees FreeDirtyTextures makes.
type freePainter struct{ gpu.Painter }

func (freePainter) Free(core.CanvasObject) {}

// TestPartialDirtyRegion_ModalScrimRefresh pins #605's scrim half: a modal
// pop-up's entrance animation fades its scrim, which covers the canvas, while
// its content is only as wide as a drawer. A refresh that repainted the
// content alone left the scrim undimmed outside the drawer's strip on an
// iPad, where the strip is too narrow for the frame to be a full repaint.
func TestPartialDirtyRegion_ModalScrimRefresh(t *testing.T) {
	if !build.PartialRepaint {
		t.Skip("the dirty region is only tracked under partial repaint")
	}
	w := d.CreateWindow("scrim").(*window)
	defer w.Close()
	c := w.Canvas().(*canvas)
	c.padded = false
	w.SetContent(belluicanvas.NewRectangle(color.White))
	c.Resize(geo.NewSize(820, 1180))
	c.SetPainter(freePainter{})

	pop := widget.NewModalPopUp(belluicanvas.NewRectangle(color.Black), c)
	pop.Resize(geo.NewSize(360, 1180))
	pop.Show()
	pop.Move(geo.NewPos(0, 0))
	c.FreeDirtyTextures()
	c.PartialDirtyRegion()
	c.FreeDirtyTextures()
	c.PartialDirtyRegion()

	pop.Refresh() // an animation step
	c.FreeDirtyTextures()
	pos, sz, set := c.PartialDirtyRegion()
	require.True(t, set)
	assert.LessOrEqual(t, pos.X, float32(0))
	assert.GreaterOrEqual(t, pos.X+sz.Width, float32(820), "the region covers the scrim, not the drawer's strip alone")
}
