package driver_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/belluikit/bellui/core"
	"github.com/belluikit/bellui/geo"
	"github.com/belluikit/bellui/internal/driver"
)

// bandOverlay covers the canvas but leaves the band above top to the content
// beneath, as a drawer below a header does (#1171).
type bandOverlay struct {
	objectTree
	top float32
}

func (o *bandOverlay) PassesInputThroughAt(pos geo.Position) bool { return pos.Y < o.top }

// Above the band's top an overlay is skipped as if absent; below it the
// overlay is the barrier a modal one is, even where it matches nothing.
func TestFindObjectAtPosition_PassesInputThroughAt(t *testing.T) {
	button := &objectTree{pos: geo.NewPos(0, 0), size: geo.NewSize(100, 40)}
	content := &objectTree{children: []core.CanvasObject{button}, size: geo.NewSize(100, 200)}
	overlay := &bandOverlay{objectTree: objectTree{size: geo.NewSize(100, 200)}, top: 40}
	isButton := func(o core.CanvasObject) bool { return o == button }

	found, _, layer := driver.FindObjectAtPositionMatching(geo.NewPos(50, 20), isButton, []core.CanvasObject{overlay}, content)
	assert.Equal(t, button, found, "above top the content gets it")
	assert.Equal(t, 1, layer)

	overlay.top = 10
	found, _, _ = driver.FindObjectAtPositionMatching(geo.NewPos(50, 20), isButton, []core.CanvasObject{overlay}, content)
	assert.Nil(t, found, "below top the overlay blocks it")

	scroller := &scrollableTree{objectTree: objectTree{size: geo.NewSize(100, 200)}}
	inside := &bandOverlay{objectTree: objectTree{children: []core.CanvasObject{scroller}, size: geo.NewSize(100, 200)}, top: 40}
	all := driver.FindAllObjectsAtPositionMatching(geo.NewPos(50, 20), func(o core.CanvasObject) bool { return o == scroller }, inside)
	assert.Empty(t, all, "above top the overlay's own scrollable is not there")
	all = driver.FindAllObjectsAtPositionMatching(geo.NewPos(50, 60), func(o core.CanvasObject) bool { return o == scroller }, inside)
	assert.Len(t, all, 1, "below top it is")
}
