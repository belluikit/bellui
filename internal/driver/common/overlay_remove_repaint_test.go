//go:build !nopartialrepaint && (partialrepaint || (!js && !wasm && !mobile && !(gl && (android || ios))))

package common

import (
	"image/color"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/belluikit/bellui/canvas"
	"github.com/belluikit/bellui/container"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/test"
)

// A removed overlay's pixels are repainted: the region after the removal
// covers where it was drawn. It was left out, so under partial repaint an
// overlay that went away stayed on screen wherever nothing beneath it
// repainted (#826: an Entry's caret handle left as a strip between two fields
// after Done).
func TestPartialDirtyRegion_RemovedOverlayIsRepainted(t *testing.T) {
	test.NewTempApp(t)

	content := container.NewWithoutLayout()
	content.Resize(geo.NewSize(200, 200))
	c := newPartialCanvas(content, geo.NewSize(200, 200))

	handle := canvas.NewCircle(color.White)
	handle.Resize(geo.NewSize(20, 20))
	handle.Move(geo.NewPos(90, 140))
	c.Overlays().Add(handle)

	_, _, set := c.PartialDirtyRegion() // paints the overlay, recording its bounds
	require.True(t, set)
	_, _, set = c.PartialDirtyRegion()
	require.False(t, set, "nothing changed, so nothing to repaint")

	c.Overlays().Remove(handle)
	pos, size, set := c.PartialDirtyRegion()
	require.True(t, set, "the removed overlay's area is not repainted")
	r := dirtyRect{pos: pos, size: size, set: true}
	require.True(t, r.Intersects(handle.Position(), handle.Size()),
		"the repaint region %v+%v misses the removed overlay at %v+%v", pos, size, handle.Position(), handle.Size())
}
